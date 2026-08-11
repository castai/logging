module github.com/castai/logging/export-experiment

go 1.26.2

replace github.com/castai/logging => ../

require (
	github.com/castai/logging v0.6.0
	github.com/sirupsen/logrus v1.9.3
)

require (
	golang.org/x/sys v0.0.0-20220715151400-c0bba94af5f8 // indirect
	golang.org/x/time v0.6.0 // indirect
)
