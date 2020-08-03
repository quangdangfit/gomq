package api

import "go.uber.org/dig"

func Inject(container *dig.Container) error {
	_ = container.Provide(NewSender)

	return nil
}
