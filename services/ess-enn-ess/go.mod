module github.com/tonyellard/ess-enn-ess

go 1.21

require (
	github.com/tonyellard/cloud-u-l8r/pkg/health v0.0.0
	gopkg.in/yaml.v3 v3.0.1
)

replace github.com/tonyellard/cloud-u-l8r/pkg/health => ../../pkg/health
