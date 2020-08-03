package api

import "go.uber.org/dig"

func Inject(container *dig.Container) error {
	_ = container.Provide(NewSender)
	_ = container.Provide(NewCron)

	return nil
}
