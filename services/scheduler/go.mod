module github.com/tonyellard/scheduler

go 1.23

require (
	github.com/tonyellard/cloud-u-l8r/pkg/activity v0.0.0
	github.com/tonyellard/cloud-u-l8r/pkg/awserrors v0.0.0
	github.com/tonyellard/cloud-u-l8r/pkg/health v0.0.0
	github.com/tonyellard/cloud-u-l8r/pkg/schedule v0.0.0
)

replace (
	github.com/tonyellard/cloud-u-l8r/pkg/activity => ../../pkg/activity
	github.com/tonyellard/cloud-u-l8r/pkg/awserrors => ../../pkg/awserrors
	github.com/tonyellard/cloud-u-l8r/pkg/health => ../../pkg/health
	github.com/tonyellard/cloud-u-l8r/pkg/schedule => ../../pkg/schedule
)
