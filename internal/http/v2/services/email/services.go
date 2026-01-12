// Package email contiene los services del dominio email flows.
package email

// Deps contiene las dependencias para crear los services email.
type Deps struct {
	Sender SenderProvider
	Config FlowsConfig
}

// Services agrupa todos los services del dominio email.
type Services struct {
	Flows FlowsService
}

// NewServices crea el agregador de services email.
func NewServices(d Deps) Services {
	return Services{
		Flows: NewFlowsService(FlowsDeps{
			Sender: d.Sender,
			Config: d.Config,
		}),
	}
}
