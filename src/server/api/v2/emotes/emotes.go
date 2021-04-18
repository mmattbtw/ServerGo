package emotes

import (
	"bufio"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/SevenTV/ServerGo/src/aws"
	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/SevenTV/ServerGo/src/discord"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/server/middleware"
	"github.com/SevenTV/ServerGo/src/utils"
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

	emotes.Post("/", middleware.UserAuthMiddleware(true), middleware.AuditRoute(func(c *fiber.Ctx) (int, []byte, *mongo.AuditLog) {
		c.Set("Content-Type", "application/json")
		usr, ok := c.Locals("user").(*mongo.User)
		if !ok {
			return 500, errInternalServer, nil
		}
		if !mongo.UserHasPermission(usr, mongo.RolePermissionEmoteCreate) {
			return 403, utils.S2B(fmt.Sprintf(errAccessDenied, "You don't have permission to do that.")), nil
		}

		req := c.Request()
		fctx := c.Context()
		if !req.IsBodyStream() {
			return 400, utils.S2B(fmt.Sprintf(errInvalidRequest, "You did not provide an upload stream.")), nil
		}

		// Get file stream
		file := fctx.RequestBodyStream()
		mr := multipart.NewReader(file, utils.B2S(req.Header.MultipartFormBoundary()))
		var emote *mongo.Emote
		var emoteName string              // The name of the emote
		var channelID *primitive.ObjectID // The channel creating this emote
		var ogFilePath string             // The original file path
		var contentType string
		id, _ := uuid.NewRandom()

		// The temp directory where the emote will be created
		fileDir := fmt.Sprintf("%s/%s", configure.Config.GetString("temp_file_store"), id.String())
		if err := os.MkdirAll(fileDir, 0777); err != nil {
			log.Errorf("mkdir, err=%v", err)
			return 500, errInternalServer, nil
		}

		// Remove temp dir once this function completes
		defer os.RemoveAll(fileDir)

		// Get form data parts
		channelID = &usr.ID // Default channel ID to the uploader
		for i := 0; i < 3; i++ {
			part, err := mr.NextPart()
			if err != nil {
				continue
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
				if emoteName == "" { // Infer emote name from file name if it wasn't specified
					basename := part.FileName()
					emoteName = strings.TrimSuffix(basename, filepath.Ext(basename))
				}
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

		if !mongo.UserHasPermission(usr, mongo.RolePermissionManageUsers) {
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

		// Convert original file into WEBP sizes
		cmd := exec.Command("ffmpeg", []string{
			"-y",
			"-i", ogFilePath,
			"-filter_complex", "scale=if(gte(iw*min(128\\,ih)/ih\\,384)\\,384\\,-1):min(128\\,ih),split=2[four][out1],[out1]scale=-1:min(76\\,ih),split=2[three][out2],[out2]scale=-1:min(48\\,ih),split=2[two][out3],[out3]scale=-1:min(32\\,ih)[one]",
			"-map", "[four]",
			"-qscale", "100",
			"-loop", "0",
			files[0][0],
			"-map", "[three]",
			"-qscale", "100",
			"-loop", "0",
			files[1][0],
			"-map", "[two]",
			"-qscale", "100",
			"-loop", "0",
			files[2][0],
			"-map", "[one]",
			"-qscale", "100",
			"-loop", "0",
			files[3][0],
		}...)

		// Print output to console for debugging
		stderr, _ := cmd.StderrPipe()
		go func() {
			scan := bufio.NewScanner(stderr) // Create a scanner tied to stdout
			fmt.Println("--- BEGIN " + cmd.String() + " CMD ---")
			for scan.Scan() { // Capture stdout, appending it to cmd var and logging to console
				fmt.Println(scan.Text())
			}
			fmt.Println("\n--- END CMD ---")
		}()
		err := cmd.Run()
		if err != nil {
			log.Errorf("cmd, err=%v", err)
			return 500, errInternalServer, nil
		}

		wg := &sync.WaitGroup{}
		wg.Add(len(files))

		mime := "image/webp"
		emote = &mongo.Emote{
			Name:             emoteName,
			Mime:             mime,
			Status:           mongo.EmoteStatusProcessing,
			Tags:             []string{},
			Visibility:       mongo.EmoteVisibilityPrivate,
			OwnerID:          *channelID,
			LastModifiedDate: time.Now(),
		}
		res, err := mongo.Database.Collection("emotes").InsertOne(mongo.Ctx, emote)

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

				if err := aws.UploadFile(configure.Config.GetString("aws_cdn_bucket"), fmt.Sprintf("emote/%s/%s", _id.Hex(), path[1]), data, &mime); err != nil {
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

		discord.SendEmoteCreate(*emote, *usr)
		return 201, utils.S2B(fmt.Sprintf(`{"status":201,"id":"%s"}`, _id.Hex())), &mongo.AuditLog{
			Type: mongo.AuditLogTypeEmoteCreate,
			Changes: []*mongo.AuditLogChange{
				{Key: "name", OldValue: nil, NewValue: emoteName},
				{Key: "tags", OldValue: nil, NewValue: []string{}},
				{Key: "owner", OldValue: nil, NewValue: usr.ID},
				{Key: "visibility", OldValue: nil, NewValue: mongo.EmoteVisibilityPrivate},
				{Key: "mime", OldValue: nil, NewValue: mime},
				{Key: "status", OldValue: nil, NewValue: mongo.EmoteStatusProcessing},
			},
			Target:    &mongo.Target{ID: &_id, Type: "emotes"},
			CreatedBy: usr.ID,
		}
	}))

	return emotes
}
