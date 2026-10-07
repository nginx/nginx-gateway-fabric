package controller

//go:generate sh -c "go tool moq -skip-ensure -pkg controllerfakes -out controllerfakes/fake_manager.go \"$(go list -f '{{.Dir}}' sigs.k8s.io/controller-runtime/pkg/manager)\" Manager"
//go:generate sh -c "go tool moq -skip-ensure -pkg controllerfakes -out controllerfakes/fake_field_indexer.go \"$(go list -f '{{.Dir}}' sigs.k8s.io/controller-runtime/pkg/client)\" FieldIndexer"
