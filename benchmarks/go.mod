module github.com/castai/logging/benchmarks

go 1.26.2

replace github.com/castai/logging => ../

require (
	github.com/castai/logging v0.0.0-00010101000000-000000000000
	github.com/sirupsen/logrus v1.9.4
)

require (
	golang.org/x/sys v0.13.0 // indirect
	golang.org/x/time v0.6.0 // indirect
)
