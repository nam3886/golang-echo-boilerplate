package cron

import "go.uber.org/fx"

// Module provides the cron scheduler to the Fx container.
var Module = fx.Module("cron",
	fx.Provide(NewScheduler),
	fx.Invoke(Start),
)
