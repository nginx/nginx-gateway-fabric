package controller

import (
	_ "sigs.k8s.io/controller-runtime/pkg/client"  // used below to generate a fake
	_ "sigs.k8s.io/controller-runtime/pkg/manager" // used below to generate a fake
)

//counterfeiter:generate  sigs.k8s.io/controller-runtime/pkg/manager.Manager

//counterfeiter:generate  sigs.k8s.io/controller-runtime/pkg/client.FieldIndexer
