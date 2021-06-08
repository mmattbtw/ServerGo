module github.com/SevenTV/ServerGo

go 1.16

require (
	github.com/SevenTV/discordgo v0.23.3-0.20210608155825-50fb0c63a471
	github.com/andybalholm/brotli v1.0.3 // indirect
	github.com/antonfisher/nested-logrus-formatter v1.3.1
	github.com/aws/aws-sdk-go v1.34.28
	github.com/bsm/redislock v0.7.0
	github.com/davecgh/go-spew v1.1.1
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/fasthttp/websocket v1.4.3-beta.1 // indirect
	github.com/go-redis/redis/v8 v8.7.1
	github.com/gobuffalo/packr/v2 v2.8.1
	github.com/gofiber/fiber/v2 v2.11.0
	github.com/gofiber/websocket/v2 v2.0.3
	github.com/google/uuid v1.2.0
	github.com/graph-gophers/graphql-go v0.0.0-20210319060855-d2656e8bde15
	github.com/json-iterator/go v1.1.10
	github.com/karrick/godirwalk v1.16.1 // indirect
	github.com/klauspost/compress v1.13.0 // indirect
	github.com/kr/pretty v0.2.1
	github.com/pasztorpisti/qs v0.0.0-20171216220353-8d6c33ee906c
	github.com/rogpeppe/go-internal v1.8.0 // indirect
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.1
	github.com/valyala/fasthttp v1.26.0 // indirect
	go.mongodb.org/mongo-driver v1.5.0
	golang.org/x/net v0.0.0-20210525063256-abc453219eb5 // indirect
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c // indirect
	golang.org/x/sys v0.0.0-20210608053332-aa57babbf139 // indirect
	golang.org/x/term v0.0.0-20210317153231-de623e64d2a6 // indirect
	gopkg.in/gographics/imagick.v3 v3.4.0
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776 // indirect
	k8s.io/apimachinery v0.21.1
	k8s.io/client-go v0.21.1
)

replace github.com/graph-gophers/graphql-go => github.com/troydota/graphql-go v0.0.0-20210513105700-d1faf5042c4f

replace github.com/gofiber/fiber/v2 => github.com/SevenTV/fiber/v2 v2.6.1-0.20210513111059-44313cd6b916
