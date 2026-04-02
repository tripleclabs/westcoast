module github.com/tripleclabs/westcoast/pkg/azureprovider

go 1.26.1

require (
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.9.0
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6 v6.3.0
	github.com/tripleclabs/westcoast v0.0.0
	github.com/tripleclabs/westcoast/pkg/providerutil v0.0.0
)

require (
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.18.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.11.1 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.4.2 // indirect
	github.com/golang-jwt/jwt/v5 v5.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	golang.org/x/crypto v0.49.0 // indirect
	golang.org/x/net v0.52.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/text v0.35.0 // indirect
)

replace (
	github.com/tripleclabs/westcoast => ../..
	github.com/tripleclabs/westcoast/pkg/providerutil => ../providerutil
)
