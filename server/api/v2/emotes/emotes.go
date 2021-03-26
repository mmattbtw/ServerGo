package emotes

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/SevenTV/ServerGo/aws"
	"github.com/SevenTV/ServerGo/configure"
	"github.com/SevenTV/ServerGo/mongo"
	"github.com/SevenTV/ServerGo/server/middleware"
	"github.com/SevenTV/ServerGo/utils"
	"github.com/gofiber/fiber/v2"
)

const chunkSize = 1024 * 1024

var (
	errInternalServer = []byte(`{"status":500,"message":"internal server error"}`)
	errInvalidRequest = `{"status":400,"message":"%s"}`
	errAccessDenied   = `{"status":403,"message":"%s"}`
)

func Emotes(app fiber.Router) fiber.Router {
	emotes := app.Group("/emotes", middleware.UserAuthMiddleware(true))

	emotes.Post("/upload", middleware.UserAuthMiddleware(true), middleware.AuditRoute(func(c *fiber.Ctx) (int, []byte, *mongo.AuditLog) {
		c.Set("Content-Type", "application/json")
		usr, ok := c.Locals("user").(*mongo.User)
		if !ok {
			return 500, errInternalServer, nil
		}

		req := c.Request()
		fctx := c.Context()
		if !req.IsBodyStream() {
			return 400, utils.S2B(fmt.Sprintf(errInvalidRequest, "You did not provide an upload stream.")), nil
		}

		file := fctx.RequestBodyStream()
		mr := multipart.NewReader(file, utils.B2S(req.Header.MultipartFormBoundary()))
		var emoteName string
		var channelID *primitive.ObjectID
		var ogFilePath string
		var contentType string
		id, _ := uuid.NewRandom()

		fileDir := fmt.Sprintf("%s/%s", configure.Config.GetString("temp_file_store"), id.String())

		if err := os.MkdirAll(fileDir, 0644); err != nil {
			log.Errorf("mkdir, err=%v", err)
			return 500, errInternalServer, nil
		}

		defer os.RemoveAll(fileDir)

		for i := 0; i < 3; i++ {
			part, err := mr.NextPart()
			if err != nil {
				return 400, utils.S2B(fmt.Sprintf(errInvalidRequest, "There are not enough fields in the form body.")), nil
			}

			if part.FormName() == "name" {
				buf := make([]byte, 32)
				n, err := part.Read(buf)
				if err != nil && err != io.EOF {
					return 400, utils.S2B(fmt.Sprintf(errInvalidRequest, "We couldn't read the name.")), nil
				}
				// max emote name length
				if n > 16 {
					return 400, utils.S2B(fmt.Sprintf(errInvalidRequest, "The emote name is longer than we expected.")), nil
				}
				emoteName = utils.B2S(buf[:n])
			} else if part.FormName() == "channel" {
				buf := make([]byte, 64)
				n, err := part.Read(buf)
				if err != nil && err != io.EOF {
					return 400, utils.S2B(fmt.Sprintf(errInvalidRequest, "We couldn't read the channel ID.")), nil
				}
				id, err := primitive.ObjectIDFromHex(utils.B2S(buf[:n]))
				if err != nil {
					return 400, utils.S2B(fmt.Sprintf(errInvalidRequest, "The channel ID is not valid.")), nil
				}
				channelID = &id
			} else if part.FormName() == "emote" {
				data := make([]byte, chunkSize)
				contentType = part.Header.Get("Content-Type")
				var ext string
				switch contentType {
				case "image/jpeg":
					ext = "jpg"
				case "image/png":
					ext = "png"
				case "image/gif":
					ext = "gif"
				case "image/webp":
					ext = "webp"
				case "image/x-canon-cr2":
					ext = "cr2"
				case "image/tiff":
					ext = "tif"
				case "image/bmp":
					ext = "bmp"
				case "image/heif":
					ext = "heif"
				case "image/vnd.ms-photo":
					ext = "jxr"
				case "image/vnd.adobe.photoshop":
					ext = "psd"
				case "image/vnd.microsoft.icon":
					ext = "ico"
				case "image/vnd.dwg":
					ext = "dwg"
				default:
					return 400, utils.S2B(fmt.Sprintf(errInvalidRequest, "The file content type is not an image.")), nil
				}

				ogFilePath = fmt.Sprintf("%s/og.%s", fileDir, ext)

				osFile, err := os.Create(ogFilePath)
				if err != nil {
					log.Errorf("file, err=%v", err)
					return 500, errInternalServer, nil
				}

				for {
					n, err := part.Read(data)
					if err != nil && err != io.EOF {
						log.Errorf("read, err=%v", err)
						return 400, utils.S2B(fmt.Sprintf(errInvalidRequest, "We failed to read the file.")), nil
					}
					_, err2 := osFile.Write(data[:n])
					if err2 != nil {
						osFile.Close()
						log.Errorf("write, err=%v", err)
						return 500, errInternalServer, nil
					}
					if err == io.EOF {
						break
					}
				}
				log.Infoln("wrote file")
				osFile.Close()
			}
		}

		if emoteName == "" || channelID == nil {
			return 400, utils.S2B(fmt.Sprintf(errInvalidRequest, "The fields were not provided.")), nil
		}

		if usr.Rank != mongo.UserRankAdmin {
			if channelID.Hex() != usr.ID.Hex() {
				if err := mongo.Database.Collection("users").FindOne(mongo.Ctx, bson.M{
					"_id":     channelID,
					"editors": usr.ID,
				}).Err(); err != nil {
					if err == mongo.ErrNoDocuments {
						return 403, utils.S2B(fmt.Sprintf(errAccessDenied, "You don't have permission to do that.")), nil
					}
					log.Errorf("mongo, err=%v", err)
					return 500, errInternalServer, nil
				}
			}
		}

		files := [][]string{
			{fmt.Sprintf("%s/4x.webp", fileDir), "4x"},
			{fmt.Sprintf("%s/3x.webp", fileDir), "3x"},
			{fmt.Sprintf("%s/2x.webp", fileDir), "2x"},
			{fmt.Sprintf("%s/1x.webp", fileDir), "1x"},
		}

		cmd := exec.Command("ffmpeg", []string{
			"-y",
			"-i", ogFilePath,
			"-filter_complex", "scale=if(gte(iw*min(128\\,ih)/ih\\,384)\\,384\\,-1):min(128\\,ih),split=2[four][out1],[out1]scale=-1:min(76\\,ih),split=2[three][out2],[out2]scale=-1:min(48\\,ih),split=2[two][out3],[out3]scale=-1:min(32\\,ih)[one]",
			"-map", "[four]",
			"-qscale", "90",
			"-loop", "0",
			files[0][0],
			"-map", "[three]",
			"-qscale", "70",
			"-loop", "0",
			files[1][0],
			"-map", "[two]",
			"-qscale", "50",
			"-loop", "0",
			files[2][0],
			"-map", "[one]",
			"-qscale", "30",
			"-loop", "0",
			files[3][0],
		}...)

		err := cmd.Run()
		if err != nil {
			log.Errorf("cmd, err=%v", err)
			return 500, errInternalServer, nil
		}

		wg := &sync.WaitGroup{}
		wg.Add(len(files))

		res, err := mongo.Database.Collection("emotes").InsertOne(mongo.Ctx, &mongo.Emote{
			Name:             emoteName,
			Mime:             "image/webp",
			Status:           mongo.EmoteStatusProcessing,
			Tags:             []string{},
			Visibility:       mongo.EmoteVisibilityPrivate,
			OwnerID:          *channelID,
			LastModifiedDate: time.Now(),
		})

		if err != nil {
			log.Errorf("mongo, err=%v", err)
			return 500, errInternalServer, nil
		}

		_id, ok := res.InsertedID.(primitive.ObjectID)
		if !ok {
			log.Errorf("mongo, id=%v", res.InsertedID)
			_, err := mongo.Database.Collection("emotes").DeleteOne(mongo.Ctx, bson.M{
				"_id": res.InsertedID,
			})
			if err != nil {
				log.Errorf("mongo, err=%v", err)
			}
			return 500, errInternalServer, nil
		}

		errored := false

		for _, path := range files {
			go func(path []string) {
				defer wg.Done()
				data, err := os.ReadFile(path[0])
				if err != nil {
					log.Errorf("read, err=%v", err)
					errored = true
					return
				}
				webp := "image/webp"
				if err := aws.UploadFile(configure.Config.GetString("aws_cdn_bucket"), fmt.Sprintf("emote/%s/%s", _id.Hex(), path[1]), data, &webp); err != nil {
					log.Errorf("aws, err=%v", err)
					errored = true
				}
			}(path)
		}

		wg.Wait()

		if errored {
			_, err := mongo.Database.Collection("emotes").DeleteOne(mongo.Ctx, bson.M{
				"_id": _id,
			})
			if err != nil {
				log.Errorf("mongo, err=%v, id=%s", err, _id.Hex())
			}
			return 500, errInternalServer, nil
		}

		_, err = mongo.Database.Collection("emotes").UpdateOne(mongo.Ctx, bson.M{
			"_id": _id,
		}, bson.M{
			"$set": bson.M{
				"status": mongo.EmoteStatusLive,
			},
		})
		if err != nil {
			log.Errorf("mongo, err=%v, id=%s", err, _id.Hex())
		}

		return 200, utils.S2B(fmt.Sprintf(`{"status":200,"id":"%s"}`, _id.Hex())), &mongo.AuditLog{
			Type:      mongo.AuditLogTypeEmoteCreate,
			Target:    &mongo.Target{ID: &_id, Type: "emotes"},
			CreatedBy: usr.ID,
		}
	}))

	return emotes
}
