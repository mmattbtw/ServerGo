package emotes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SevenTV/ServerGo/src/aws"
	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/SevenTV/ServerGo/src/discord"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/server/api/v2/rest/restutil"
	"github.com/SevenTV/ServerGo/src/server/middleware"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/SevenTV/ServerGo/src/validation"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	imConfigure "github.com/seventv/ImageProcessor/src/configure"
	"github.com/seventv/ImageProcessor/src/containers"
	"github.com/seventv/ImageProcessor/src/global"
	"github.com/seventv/ImageProcessor/src/image"
	"github.com/seventv/ImageProcessor/src/job"
	"github.com/seventv/ImageProcessor/src/task"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	MAX_FRAME_COUNT  = 4096
	MAX_FILE_SIZE    = 2500000
	MAX_PIXEL_HEIGHT = 1000
	MAX_PIXEL_WIDTH  = 1000
	MAX_FRAMES       = 750
	RATE_LIMIT       = 15
)

var webpMuxRegex = regexp.MustCompile(`Canvas size: (\d+) x (\d+)(?:\n?.*\n){0,2}(?:Number of frames: (\d+))?`) // capture group 1: width, 2: height, 3: frame count or empty which means 1

func CreateEmoteRoute(router fiber.Router) {
	rateLimiter := make(chan struct{}, RATE_LIMIT)
	for i := 0; i < RATE_LIMIT; i++ {
		rateLimiter <- struct{}{}
	}

	gCtx := global.New(context.TODO(), &imConfigure.Config{})

	rl := configure.Config.GetIntSlice("limits.route.emote-create")
	router.Post(
		"/",
		middleware.UserAuthMiddleware(true),
		middleware.RateLimitMiddleware("emote-create", int32(rl[0]), time.Millisecond*time.Duration(rl[1])),
		func(c *fiber.Ctx) error {
			start := time.Now()

			c.Set("Content-Type", "application/json")
			usr, ok := c.Locals("user").(*datastructure.User)
			if !ok {
				return restutil.ErrLoginRequired().Send(c)
			}
			if !usr.HasPermission(datastructure.RolePermissionEmoteCreate) {
				return restutil.ErrAccessDenied().Send(c)
			}

			<-rateLimiter
			defer func() {
				rateLimiter <- struct{}{}
			}()
			if time.Since(start) > time.Second*10 {
				logrus.Warn("upload endpoint flooded")
				return restutil.ErrInternalServer().Send(c, "Endpoint too flooded...")
			}

			req := c.Request()
			fctx := c.Context()
			if !req.IsBodyStream() {
				return restutil.ErrBadRequest().Send(c, "Not A File Stream")
			}

			// Get file stream
			file := fctx.RequestBodyStream()
			mr := multipart.NewReader(file, utils.B2S(req.Header.MultipartFormBoundary()))

			var (
				emote           *datastructure.Emote
				emoteName       string              // The name of the emote
				emoteVisibility int32               // The starting visibility for the emote
				emoteTags       []string            // The emote's tags, if any
				channelID       *primitive.ObjectID // The channel creating this emote
				imgType         image.ImageType
				ogFilePath      string
			)

			tmpUuid, _ := uuid.NewRandom()

			// The temp directory where the emote will be created
			tmpDir := path.Join(configure.Config.GetString("temp_file_store"), tmpUuid.String())
			if err := os.MkdirAll(tmpDir, 0777); err != nil {
				logrus.WithError(err).Error("mkdir")
				return restutil.ErrInternalServer().Send(c)
			}
			defer os.RemoveAll(tmpDir)

			// Get form data parts
			channelID = &usr.ID // Default channel ID to the uploader
			for {
				part, err := mr.NextPart()
				if err == io.EOF {
					break
				} else if err != nil {
					logrus.WithError(err).Error("multipart_reader")
					break
				}

				switch part.FormName() {
				case "name":
					buf := make([]byte, 100)
					n, err := part.Read(buf)
					if err != nil && err != io.EOF {
						return restutil.ErrBadRequest().Send(c, "Emote Name Not Readable")
					}
					emoteName = utils.B2S(buf[:n])
				case "tags":
					b, err := io.ReadAll(part)
					if err != nil {
						return restutil.ErrBadRequest().Send(c, "Couldn't parse tags")
					}
					if len(b) == 0 {
						continue
					}

					emoteTags = strings.Split(utils.B2S(b), ",")
					// Validate tags
					if len(emoteTags) > 6 {
						return restutil.ErrBadRequest().Send(c, "Too Many Tags (6)")
					}
					if ok, badTag := validation.ValidateEmoteTags(emoteTags); !ok {
						return restutil.ErrBadRequest().Send(c, fmt.Sprintf("'%s' is not a valid tag", badTag))
					}
				case "visibility":
					b, err := io.ReadAll(part)
					if err != nil {
						return restutil.ErrBadRequest().Send(c, "Invalid Visibility Flags")
					}
					if len(b) == 0 {
						continue
					}

					i, err := strconv.Atoi(utils.B2S(b))
					if err != nil {
						return restutil.ErrBadRequest().Send(c, fmt.Sprintf("Could not parse visibility: %s", err.Error()))
					}
					if utils.BitField.HasBits(int64(i), int64(datastructure.EmoteVisibilityPrivate)) {
						emoteVisibility |= datastructure.EmoteVisibilityPrivate
					}
					if utils.BitField.HasBits(int64(i), int64(datastructure.EmoteVisibilityZeroWidth)) {
						emoteVisibility |= datastructure.EmoteVisibilityZeroWidth
					}
				case "channel":
					buf := make([]byte, 64)
					n, err := part.Read(buf)
					if err != nil && err != io.EOF {
						return restutil.ErrBadRequest().Send(c, "User ID Not Readable")
					}
					id, err := primitive.ObjectIDFromHex(utils.B2S(buf[:n]))
					if err != nil {
						return restutil.ErrBadRequest().Send(c, "Invalid User ID")
					}
					channelID = &id
				case "emote":
					if emoteName == "" { // Infer emote name from file name if it wasn't specified
						basename := part.FileName()
						emoteName = strings.TrimSuffix(basename, filepath.Ext(basename))
					}

					data, err := ioutil.ReadAll(part)
					if err != nil {
						logrus.WithError(err).Error("file")
						return restutil.ErrInternalServer().Send(c)
					}

					imgType, err = containers.ToType(data)
					if err != nil {
						return restutil.ErrBadRequest().Send(c, "Whatever you uploaded it aint no image.")
					}

					if len(data) >= MAX_FILE_SIZE {
						return restutil.ErrBadRequest().Send(c, fmt.Sprintf("Input File Too Large. Must be <%vMB", MAX_FILE_SIZE/1000000))
					}

					ogFilePath = path.Join(tmpDir, fmt.Sprintf("og.%s", imgType))
					if err := os.WriteFile(ogFilePath, data, 0700); err != nil {
						logrus.WithError(err).Error("read")
						return restutil.ErrBadRequest().Send(c, "File Not Readable")
					}
				}
			}

			if emoteName == "" || channelID == nil {
				return restutil.ErrBadRequest().Send(c, "Uncomplete Form")
			}

			if !validation.ValidateEmoteName(utils.S2B(emoteName)) {
				return restutil.ErrBadRequest().Send(c, "Invalid Emote Name")
			}

			if !usr.HasPermission(datastructure.RolePermissionManageUsers) && channelID.Hex() != usr.ID.Hex() {
				if err := mongo.Collection(mongo.CollectionNameUsers).FindOne(c.Context(), bson.M{
					"_id":     channelID,
					"editors": usr.ID,
				}).Err(); err != nil {
					if err == mongo.ErrNoDocuments {
						return restutil.ErrAccessDenied().Send(c)
					}
					logrus.WithError(err).Error("mongo")
					return restutil.ErrInternalServer().Send(c)
				}
			}

			var (
				width      int
				height     int
				frameCount int
			)

			switch imgType {
			case image.AVI, image.AVIF, image.FLV, image.MP4, image.WEBM, image.GIF, image.JPEG, image.PNG, image.TIFF:
				// use ffprobe to get the number of frames and width/height
				// ffprobe -v error -select_streams v:0 -count_frames -show_entries stream=nb_read_frames,width,height -of csv=p=0 file.ext

				output, err := exec.CommandContext(c.Context(),
					"ffprobe",
					"-v", "fatal",
					"-select_streams", "v:0",
					"-count_frames",
					"-show_entries",
					"stream=nb_read_frames,width,height",
					"-of", "csv=p=0",
					ogFilePath,
				).Output()
				if err != nil {
					logrus.WithError(err).Error("failed to run ffprobe command")
					return restutil.ErrInternalServer().Send(c)
				}

				splits := strings.Split(strings.TrimSpace(utils.B2S(output)), ",")
				if len(splits) != 3 {
					logrus.Errorf("ffprobe command returned bad results: %s", output)
					return restutil.ErrInternalServer().Send(c)
				}

				width, err = strconv.Atoi(splits[0])
				if err != nil {
					logrus.WithError(err).Errorf("ffprobe command returned bad results: %s", output)
					return restutil.ErrInternalServer().Send(c)
				}

				height, err = strconv.Atoi(splits[1])
				if err != nil {
					logrus.WithError(err).Errorf("ffprobe command returned bad results: %s", output)
					return restutil.ErrInternalServer().Send(c)
				}

				frameCount, err = strconv.Atoi(splits[2])
				if err != nil {
					logrus.WithError(err).Errorf("ffprobe command returned bad results: %s", output)
					return restutil.ErrInternalServer().Send(c)
				}
			case image.WEBP:
				// use a webpmux -info to get the frame count and width/height
				output, err := exec.CommandContext(c.Context(),
					"webpmux",
					"-info",
					ogFilePath,
				).Output()
				if err != nil {
					logrus.WithError(err).Error("failed to run webpmux command")
					return restutil.ErrInternalServer().Send(c)
				}

				matches := webpMuxRegex.FindAllStringSubmatch(utils.B2S(output), 1)
				if len(matches) == 0 {
					logrus.Errorf("webpmux command returned bad results: %s", output)
					return restutil.ErrInternalServer().Send(c)
				}

				width, err = strconv.Atoi(matches[0][1])
				if err != nil {
					logrus.WithError(err).Errorf("ffprobe command returned bad results: %s", output)
					return restutil.ErrInternalServer().Send(c)
				}

				height, err = strconv.Atoi(matches[0][2])
				if err != nil {
					logrus.WithError(err).Errorf("ffprobe command returned bad results: %s", output)
					return restutil.ErrInternalServer().Send(c)
				}

				if matches[0][3] != "" {
					frameCount, err = strconv.Atoi(matches[0][3])
					if err != nil {
						logrus.WithError(err).Errorf("ffprobe command returned bad results: %s", output)
						return restutil.ErrInternalServer().Send(c)
					}
				} else {
					frameCount = 1
				}
			}

			if width > MAX_PIXEL_WIDTH || height > MAX_PIXEL_HEIGHT {
				return restutil.ErrBadRequest().Send(c, fmt.Sprintf("Too Many Pixels (maximum %dx%d)", MAX_PIXEL_WIDTH, MAX_PIXEL_HEIGHT))
			}

			if width <= 0 || height <= 0 {
				return restutil.ErrBadRequest().Send(c, "Bad Image")
			}

			if frameCount > MAX_FRAMES {
				return restutil.ErrBadRequest().Send(c, fmt.Sprintf("Too many Frames (maximum %d)", MAX_FRAMES))
			}

			mime := "image/webp"
			id := primitive.NewObjectIDFromTimestamp(time.Now())

			rawDetails, _ := json.Marshal(job.RawProviderDetailsLocal{
				Path: ogFilePath,
			})

			resultDetails, _ := json.Marshal(job.ResultConsumerDetailsLocal{
				PathFolder: tmpDir,
			})

			arXY := []int{3, 1}
			sizes := map[string]job.ImageSize{
				"1x": {
					Width:  32 * 3,
					Height: 32,
				},
				"2x": {
					Width:  64 * 3,
					Height: 64,
				},
				"3x": {
					Width:  96 * 3,
					Height: 96,
				},
				"4x": {
					Width:  128 * 3,
					Height: 128,
				},
			}

			job := job.Job{
				ID: id.Hex(),

				AspectRatioXY: arXY,
				Settings:      job.EnableOutputAnimatedWEBP | job.EnableOutputStaticWEBP,
				Sizes:         sizes,

				RawProvider:           job.LocalProvider,
				RawProviderDetails:    rawDetails,
				ResultConsumer:        job.LocalConsumer,
				ResultConsumerDetails: resultDetails,
			}

			task := task.New(c.Context(), job)

			task.Start(gCtx)
			for range task.Events() {
			}
			<-task.Done()
			if err := task.Failed(); err != nil {
				logrus.WithError(err).Error("failed to process emote")
				return restutil.ErrInternalServer().Send(c, "Failed to process emote")
			}

			files := task.Files()
			widths := [4]int16{}
			heights := [4]int16{}
			for _, file := range files {
				switch file.Name {
				case "1x.webp":
					widths[0] = int16(file.Width)
					heights[0] = int16(file.Height)
				case "2x.webp":
					widths[1] = int16(file.Width)
					heights[1] = int16(file.Height)
				case "3x.webp":
					widths[2] = int16(file.Width)
					heights[2] = int16(file.Height)
				case "4x.webp":
					widths[3] = int16(file.Width)
					heights[3] = int16(file.Height)
				}
			}

			emotePath := path.Join("emote", id.Hex())
			wg := sync.WaitGroup{}
			wg.Add(4)
			errCh := make(chan error, 4)
			for i := 1; i <= 4; i++ {
				go func(i int) {
					defer wg.Done()

					file, err := os.OpenFile(path.Join(tmpDir, fmt.Sprintf("%dx.webp", i)), os.O_RDONLY, 0700)
					if err != nil {
						errCh <- err
						return
					}

					errCh <- aws.UploadFile(configure.Config.GetString("aws_cdn_bucket"), path.Join(emotePath, fmt.Sprintf("%dx", i)), file, &mime)
				}(i)
			}

			var tErr error
			wg.Wait()
			for i := 0; i < 4; i++ {
				tErr = multierror.Append(tErr, <-errCh).ErrorOrNil()
			}
			close(errCh)

			if tErr != nil {
				logrus.WithError(tErr).Error("failed to upload to aws")
				return restutil.ErrInternalServer().Send(c)
			}

			emote = &datastructure.Emote{
				ID:               id,
				Name:             emoteName,
				Mime:             mime,
				Status:           datastructure.EmoteStatusLive,
				Tags:             utils.Ternary(emoteTags != nil, emoteTags, []string{}).([]string),
				Visibility:       emoteVisibility | datastructure.EmoteVisibilityUnlisted,
				OwnerID:          *channelID,
				LastModifiedDate: time.Now(),
				Width:            widths,
				Height:           heights,
				Animated:         frameCount > 1,
			}

			if _, err := mongo.Collection(mongo.CollectionNameEmotes).InsertOne(c.Context(), emote); err != nil {
				logrus.WithError(err).Error("mongo")
				return restutil.ErrInternalServer().Send(c)
			}

			_, err := mongo.Collection(mongo.CollectionNameAudit).InsertOne(c.Context(), &datastructure.AuditLog{
				Type: datastructure.AuditLogTypeEmoteCreate,
				Changes: []*datastructure.AuditLogChange{
					{Key: "name", OldValue: nil, NewValue: emoteName},
					{Key: "tags", OldValue: nil, NewValue: []string{}},
					{Key: "owner", OldValue: nil, NewValue: usr.ID},
					{Key: "visibility", OldValue: nil, NewValue: datastructure.EmoteVisibilityPrivate},
					{Key: "mime", OldValue: nil, NewValue: mime},
					{Key: "status", OldValue: nil, NewValue: datastructure.EmoteStatusProcessing},
				},
				Target:    &datastructure.Target{ID: &id, Type: "emotes"},
				CreatedBy: usr.ID,
			})
			if err != nil {
				logrus.WithError(err).Error("mongo")
			}

			go discord.SendEmoteCreate(*emote, *usr)
			return c.SendString(fmt.Sprintf(`{"id":"%v"}`, emote.ID.Hex()))
		})
}
