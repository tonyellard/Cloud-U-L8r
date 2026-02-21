module github.com/tonyellard/ess-queue-ess

go 1.23

require (
	github.com/go-chi/chi/v5 v5.2.4
	github.com/google/uuid v1.6.0
	github.com/tonyellard/cloud-u-l8r/pkg/awserrors v0.0.0
	github.com/tonyellard/cloud-u-l8r/pkg/health v0.0.0
	gopkg.in/yaml.v3 v3.0.1
)

replace (
	github.com/tonyellard/cloud-u-l8r/pkg/awserrors => ../../pkg/awserrors
	github.com/tonyellard/cloud-u-l8r/pkg/health => ../../pkg/health
)
