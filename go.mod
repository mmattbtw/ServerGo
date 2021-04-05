module github.com/SevenTV/ServerGo

go 1.16

require (
	github.com/antonfisher/nested-logrus-formatter v1.3.1
	github.com/aws/aws-sdk-go v1.34.28
	github.com/davecgh/go-spew v1.1.1
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/go-redis/redis/v8 v8.7.1
	github.com/gobuffalo/packr/v2 v2.8.1
	github.com/gofiber/fiber/v2 v2.6.0
	github.com/google/uuid v1.2.0
	github.com/graph-gophers/graphql-go v0.0.0-20210319060855-d2656e8bde15
	github.com/json-iterator/go v1.1.10
	github.com/karrick/godirwalk v1.16.1 // indirect
	github.com/kr/pretty v0.2.1
	github.com/pasztorpisti/qs v0.0.0-20171216220353-8d6c33ee906c
	github.com/rogpeppe/go-internal v1.8.0 // indirect
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.1
	go.mongodb.org/mongo-driver v1.5.0
	golang.org/x/crypto v0.0.0-20210322153248-0c34fe9e7dc2 // indirect
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c // indirect
	golang.org/x/sys v0.0.0-20210324051608-47abb6519492 // indirect
	golang.org/x/term v0.0.0-20210317153231-de623e64d2a6 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)

replace github.com/graph-gophers/graphql-go => github.com/StreamerVods/graphql-go v0.0.0-20210313100256-379b9c062e58

replace github.com/gofiber/fiber/v2 => github.com/SevenTV/fiber/v2 v2.6.1-0.20210319160543-361927aee8a2
