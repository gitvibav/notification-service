package delivery

import (
	"context"
	"sync"

	"notification-service/internal/config"
	"notification-service/internal/storage"

	"github.com/rs/zerolog"
)

type Manager struct {
	workers []*Worker
	storage storage.Storage
	logger  *zerolog.Logger
	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewManager(cfg *config.Config, strg storage.Storage) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	manager := &Manager{
		storage: strg,
		logger:  &cfg.Logger,
		ctx:     ctx,
		cancel:  cancel,
	}

	// Create workers for each channel
	workers := make([]*Worker, 0, 3)

	// Email worker
	emailSender := NewMockSender(storage.ChannelEmail, cfg.Channels.Email.FailureRate, &cfg.Logger)
	workers = append(workers, NewWorker(
		storage.ChannelEmail,
		emailSender,
		strg,
		&cfg.Channels.Email,
		&cfg.Logger,
	))

	// SMS worker
	smsSender := NewMockSender(storage.ChannelSMS, cfg.Channels.SMS.FailureRate, &cfg.Logger)
	workers = append(workers, NewWorker(
		storage.ChannelSMS,
		smsSender,
		strg,
		&cfg.Channels.SMS,
		&cfg.Logger,
	))

	// Push worker
	pushSender := NewMockSender(storage.ChannelPush, cfg.Channels.Push.FailureRate, &cfg.Logger)
	workers = append(workers, NewWorker(
		storage.ChannelPush,
		pushSender,
		strg,
		&cfg.Channels.Push,
		&cfg.Logger,
	))

	manager.workers = workers
	return manager
}

func (m *Manager) Start(notifications <-chan *storage.Notification) {
	m.logger.Info().Msg("Starting delivery manager")

	// Create separate channels for each worker type
	emailChan := make(chan *storage.Notification, 100)
	smsChan := make(chan *storage.Notification, 100)
	pushChan := make(chan *storage.Notification, 100)

	// Start workers
	for _, worker := range m.workers {
		var ch chan *storage.Notification
		switch worker.channel {
		case storage.ChannelEmail:
			ch = emailChan
		case storage.ChannelSMS:
			ch = smsChan
		case storage.ChannelPush:
			ch = pushChan
		}

		m.wg.Add(1)
		go func(w *Worker, workerChan chan *storage.Notification) {
			defer m.wg.Done()
			w.Start(m.ctx, workerChan)
		}(worker, ch)
	}

	// Start router goroutine
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer close(emailChan)
		defer close(smsChan)
		defer close(pushChan)

		m.routeNotifications(notifications, emailChan, smsChan, pushChan)
	}()
}

func (m *Manager) routeNotifications(
	input <-chan *storage.Notification,
	emailChan chan<- *storage.Notification,
	smsChan chan<- *storage.Notification,
	pushChan chan<- *storage.Notification,
) {
	for {
		select {
		case <-m.ctx.Done():
			return

		case notification, ok := <-input:
			if !ok {
				return
			}

			// Route to appropriate channel
			switch notification.Channel {
			case storage.ChannelEmail:
				select {
				case emailChan <- notification:
				case <-m.ctx.Done():
					return
				}
			case storage.ChannelSMS:
				select {
				case smsChan <- notification:
				case <-m.ctx.Done():
					return
				}
			case storage.ChannelPush:
				select {
				case pushChan <- notification:
				case <-m.ctx.Done():
					return
				}
			default:
				m.logger.Error().
					Str("notification_id", notification.ID).
					Str("channel", string(notification.Channel)).
					Msg("Unknown channel, dropping notification")
			}
		}
	}
}

func (m *Manager) Shutdown(ctx context.Context) error {
	m.logger.Info().Msg("Shutting down delivery manager")

	// Signal shutdown
	m.cancel()

	// Wait for workers to finish or timeout
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		m.logger.Info().Msg("All workers shut down gracefully")
		return nil
	case <-ctx.Done():
		m.logger.Warn().Msg("Shutdown timeout reached")
		return ctx.Err()
	}
}
