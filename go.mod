module github.com/DataDog/datadog-go/v5

go 1.13

require (
	github.com/Microsoft/go-winio v0.5.0
	github.com/golang/mock v1.6.0
	github.com/stretchr/testify v1.8.1
)

replace github.com/sirupsen/logrus v1.7.0 => github.com/sirupsen/logrus v1.9.3

// Bump vulnerable transitive dep of golang/mock; module-graph only, not compiled.
replace golang.org/x/crypto => golang.org/x/crypto v0.17.0

// Pin to a version compatible with Go 1.13.
replace golang.org/x/sys => golang.org/x/sys v0.0.0-20220715151400-c0bba94af5f8
