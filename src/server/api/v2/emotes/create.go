package emotes

import (
	"bufio"
	"bytes"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"mime/multipart"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SevenTV/ServerGo/src/aws"
	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/SevenTV/ServerGo/src/discord"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/server/middleware"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/image/draw"
)

const MAX_FRAME_COUNT int = 1024

func CreateRoute(router fiber.Router) {
	router.Post("/", middleware.UserAuthMiddleware(true), middleware.AuditRoute(func(c *fiber.Ctx) (int, []byte, *datastructure.AuditLog) {
		c.Set("Content-Type", "application/json")
		usr, ok := c.Locals("user").(*datastructure.User)
		if !ok {
			return 500, errInternalServer, nil
		}
		if !datastructure.UserHasPermission(usr, datastructure.RolePermissionEmoteCreate) {
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
		var emote *datastructure.Emote
		var emoteName string              // The name of the emote
		var channelID *primitive.ObjectID // The channel creating this emote
		var ogFileStream bytes.Buffer     // The original file, being streamed in
		var contentType string
		var ext string
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
				if n > 100 {
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
				switch contentType {
				case "image/jpeg":
					ext = "jpg"
				case "image/png":
					ext = "png"
				case "image/gif":
					ext = "gif"
				default:
					return 400, utils.S2B(fmt.Sprintf(errInvalidRequest, "The file content type is not supported. It must be one of jpg, png or gif")), nil
				}

				for {
					n, err := part.Read(data)
					if err != nil && err != io.EOF {
						log.Errorf("read, err=%v", err)
						return 400, utils.S2B(fmt.Sprintf(errInvalidRequest, "We failed to read the file.")), nil
					}

					_, err2 := ogFileStream.Write(data[:n])
					if err2 != nil {
						ogFileStream.Reset()
						log.Errorf("write, err=%v", err)
						return 500, errInternalServer, nil
					}
					if err == io.EOF {
						break
					}
				}
			}
		}

		if emoteName == "" || channelID == nil {
			return 400, utils.S2B(fmt.Sprintf(errInvalidRequest, "The fields were not provided.")), nil
		}

		if !datastructure.UserHasPermission(usr, datastructure.RolePermissionManageUsers) {
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

		// Get uploaded image file into an image.Image
		var frames []*image.Image
		var gifOptions gif.GIF
		isGIF := false

		switch ext {
		case "jpg":
			img, err := jpeg.Decode(&ogFileStream)
			if err != nil {
				log.Errorf("could not decode jpeg, err=%v", err)
				return 500, errInternalServer, nil
			}

			frames = append(frames, &img)
		case "png":
			img, err := png.Decode(&ogFileStream)
			if err != nil {
				log.Errorf("could not decode png, err=%v", err)
				return 500, errInternalServer, nil
			}

			frames = append(frames, &img)
		case "gif":
			g, err := gif.DecodeAll(&ogFileStream)
			if err != nil {
				log.Errorf("could not decode gif, err=%v", err)
				return 500, errInternalServer, nil
			}

			// Set a cap on how many frames are allowed
			if len(g.Image) > MAX_FRAME_COUNT {
				return 400, utils.S2B(fmt.Sprintf(errInvalidRequest, fmt.Sprintf("Your GIF exceeds the maximum amount of frames permitted. (%v)", MAX_FRAME_COUNT))), nil
			}

			for _, f := range g.Image {
				img := f.SubImage(f.Rect)
				frames = append(frames, &img)
			}

			isGIF = true
			gifOptions = *g
		}

		// Define sizes to be generated
		files := [][]string{
			{fmt.Sprintf("%s/4x", fileDir), "4x", "384x128", "90"},
			{fmt.Sprintf("%s/3x", fileDir), "3x", "228x76", "80"},
			{fmt.Sprintf("%s/2x", fileDir), "2x", "144x48", "75"},
			{fmt.Sprintf("%s/1x", fileDir), "1x", "96x32", "65"},
		}

		// Resize the frame(s)
		firstFrame := *frames[0]
		for _, file := range files {
			scopedFolderPath := file[0] // The temporary filepath where to store the generated emote sizes
			scope := file[1]
			sizes := strings.Split(file[2], "x")
			maxWidth, _ := strconv.ParseFloat(sizes[0], 4)
			maxHeight, _ := strconv.ParseFloat(sizes[1], 4)
			quality := file[3]

			// Get calculed ratio for the size
			width, height := utils.GetSizeRatio(
				[]float64{float64(firstFrame.Bounds().Dx()), float64(firstFrame.Bounds().Dy())},
				[]float64{maxWidth, maxHeight},
			)

			// Create new boundaries for frames
			rect := image.Rect(0, 0, int(width), int(height))

			// Render all frames individually
			_ = os.Mkdir(scopedFolderPath, 0777)

			cmdArgs := []string{ // The argument list for the img2webp command
				"-loop",
				fmt.Sprintf("%v", utils.Ternary(isGIF, gifOptions.LoopCount, 1)),
			}

			// Iterate through each frame
			// Encode as PNG, then append to arguments for webp creation
			for i, f := range frames {
				scale := draw.BiLinear
				dst := image.NewRGBA(rect) // Create new RGBA image

				// Scale the image according to our defined bounds
				// calculated by GetSizeRatio() method
				scale.Scale(dst, rect, *f, (*f).Bounds(), draw.Over, nil)
				filePath := scopedFolderPath + fmt.Sprintf("/%v.png", i)

				// Write frames to file
				writer, _ := os.Create(filePath)
				_ = png.Encode(writer, dst)

				// Append argument
				cmdArgs = append(cmdArgs, []string{
					"-lossy",
					"-q", quality,
					"-d",
					func() string {
						if isGIF && len(gifOptions.Delay) > 0 {
							return fmt.Sprint(math.Max(16, float64(gifOptions.Delay[i]*10))) // The delay is gif frame delay * 10
						} else {
							return "16"
						}
					}(),
					filePath,
				}...)
			}
			cmdArgs = append(cmdArgs, []string{ // Add outpt argument
				"-o",
				fileDir + fmt.Sprintf("/%v.webp", scope), // outputs as tmp/<uuid>/*x.webp
			}...)

			//
			cmd := exec.Command("img2webp", cmdArgs...)

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
			err := cmd.Run() // Run the command
			if err != nil {
				log.Errorf("cmd, err=%v", err)
				return 500, errInternalServer, nil
			}

		}

		wg := &sync.WaitGroup{}
		wg.Add(len(files))

		mime := "image/webp"
		emote = &datastructure.Emote{
			Name:             emoteName,
			Mime:             mime,
			Status:           datastructure.EmoteStatusProcessing,
			Tags:             []string{},
			Visibility:       datastructure.EmoteVisibilityPrivate,
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
				data, err := os.ReadFile(path[0] + ".webp")
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
				"status": datastructure.EmoteStatusLive,
			},
		})
		if err != nil {
			log.Errorf("mongo, err=%v, id=%s", err, _id.Hex())
		}

		go discord.SendEmoteCreate(*emote, *usr)
		return 201, utils.S2B(fmt.Sprintf(`{"status":201,"id":"%s"}`, _id.Hex())), &datastructure.AuditLog{
			Type: datastructure.AuditLogTypeEmoteCreate,
			Changes: []*datastructure.AuditLogChange{
				{Key: "name", OldValue: nil, NewValue: emoteName},
				{Key: "tags", OldValue: nil, NewValue: []string{}},
				{Key: "owner", OldValue: nil, NewValue: usr.ID},
				{Key: "visibility", OldValue: nil, NewValue: datastructure.EmoteVisibilityPrivate},
				{Key: "mime", OldValue: nil, NewValue: mime},
				{Key: "status", OldValue: nil, NewValue: datastructure.EmoteStatusProcessing},
			},
			Target:    &datastructure.Target{ID: &_id, Type: "emotes"},
			CreatedBy: usr.ID,
		}
	}))
}
