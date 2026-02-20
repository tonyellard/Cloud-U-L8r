module github.com/tony/ess-three

go 1.23

require (
	github.com/go-chi/chi/v5 v5.2.4
	github.com/tonyellard/cloud-u-l8r/pkg/awserrors v0.0.0
	github.com/tonyellard/cloud-u-l8r/pkg/health v0.0.0
)

replace (
	github.com/tonyellard/cloud-u-l8r/pkg/awserrors => ../../pkg/awserrors
	github.com/tonyellard/cloud-u-l8r/pkg/health => ../../pkg/health
)
