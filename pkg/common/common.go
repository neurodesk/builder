package common

type PackageManager string

const (
	PkgManagerApt PackageManager = "apt"
	PkgManagerYum PackageManager = "yum"
)
