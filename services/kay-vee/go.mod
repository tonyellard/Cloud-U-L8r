module github.com/tonyellard/kay-vee

go 1.23

require (
	github.com/tonyellard/cloud-u-l8r/pkg/awserrors v0.0.0
	github.com/tonyellard/cloud-u-l8r/pkg/health v0.0.0
)

replace (
	github.com/tonyellard/cloud-u-l8r/pkg/awserrors => ../../pkg/awserrors
	github.com/tonyellard/cloud-u-l8r/pkg/health => ../../pkg/health
)
