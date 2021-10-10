package users

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/gif"
	"io"
	"mime/multipart"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"golang.org/x/image/draw"

	"github.com/google/uuid"
	"github.com/sizeofint/webpanimation"

	"github.com/SevenTV/ServerGo/src/aws"
	"github.com/SevenTV/ServerGo/src/configure"
	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/SevenTV/ServerGo/src/mongo/datastructure"
	"github.com/SevenTV/ServerGo/src/server/api/v2/rest/restutil"
	"github.com/SevenTV/ServerGo/src/server/middleware"
	"github.com/SevenTV/ServerGo/src/utils"
	"github.com/gofiber/fiber/v2"
	log "github.com/sirupsen/logrus"
)

const MAX_PIXEL_HEIGHT = 1000
const MAX_PIXEL_WIDTH = 1000
const MIN_PIXEL_HEIGHT = 64
const MIN_PIXEL_WIDTH = 64
const MAX_UPLOAD_SIZE = 2621440    // 2.5MB
const MAX_LOSSLESS_SIZE = 256000.0 // 250KB
const QUALITY_AT_MAX_SIZE = 80.0   // A file at 2.5MB will encode at quality 80

const QUALITY_FACTOR = -((QUALITY_AT_MAX_SIZE / 100) * (MAX_LOSSLESS_SIZE - MAX_UPLOAD_SIZE)) / (1 - (QUALITY_AT_MAX_SIZE / 100))

func EditProfilePicture(router fiber.Router) {
	router.Post("/profile-picture", middleware.UserAuthMiddleware(true), func(c *fiber.Ctx) error {
		c.Set("Content-Type", "application/json")
		req := c.Request()
		ctx := c.Context()
		if !req.IsBodyStream() {
			return restutil.ErrBadRequest().Send(c, "Not A File Stream")
		}
		stream := ctx.RequestBodyStream()
		user := c.Locals("user").(*datastructure.User)
		if !user.HasPermission(datastructure.RolePermissionUseCustomAvatars) {
			return restutil.ErrAccessDenied().Send(c, "Insufficient Privilege")
		}
		if strings.HasPrefix(user.ProfileImageURL, "https://static-cdn.jtvnw.net/user-default-pictures-uv") {
			return restutil.ErrBadRequest().Send(c, "For technical reasons, you must also have a profile picture set on Twitch before you can enable an animated avatar. Please sign out and back into 7TV after doing this.")
		}

		// Read file
		var file *bytes.Reader
		mr := multipart.NewReader(stream, utils.B2S(req.Header.MultipartFormBoundary()))
		var filelength int
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			} else if err != nil {
				log.WithError(err).Error("multipart")
				break
			}
			if part.FormName() != "file" {
				return restutil.ErrBadRequest().Send(c, "Form Data Invalid")
			}

			b, err := io.ReadAll(part)
			if err != nil {
				log.WithError(err).Warn("EditProfilePicture, ReadAll")
				return restutil.ErrBadRequest().Send(c, "File Unreadable")
			}
			if len(b) > MAX_UPLOAD_SIZE {
				return restutil.ErrBadRequest().Send(c, "Input File Too Large. Must be <2.5MB")
			}

			filelength = len(b)
			file = bytes.NewReader(b)
		}

		// Decode GIF
		gif, err := gif.DecodeAll(file)
		if err != nil {
			log.WithError(err).Warn("EditProfilePicture, gif, DecodeAll")
			return restutil.ErrBadRequest().Send(c, "Invalid GIF File")
		}
		if gif.Config.Width > MAX_PIXEL_WIDTH || gif.Config.Height > MAX_PIXEL_HEIGHT {
			return restutil.ErrBadRequest().Send(c, fmt.Sprintf("Too Many Pixels (maximum %dx%d)", MAX_PIXEL_WIDTH, MAX_PIXEL_HEIGHT))
		} else if gif.Config.Width < MIN_PIXEL_WIDTH || gif.Config.Height < MIN_PIXEL_HEIGHT {
			return restutil.ErrBadRequest().Send(c, fmt.Sprintf("Too Few Pixels (mimimum %dx%d)", MIN_PIXEL_WIDTH, MIN_PIXEL_HEIGHT))
		}

		rw, rh := utils.GetSizeRatio(
			[]float64{float64(gif.Config.Width), float64(gif.Config.Height)},
			[]float64{128, 128},
		)

		// Create WebP Animation
		anim := webpanimation.NewWebpAnimation(int(rw), int(rh), gif.LoopCount)
		anim.WebPAnimEncoderOptions.SetKmin(3)
		anim.WebPAnimEncoderOptions.SetKmax(5)
		defer anim.ReleaseMemory()

		// Scale quality between QUALITY_AT_MAX_SIZE and 100
		var quality float32
		q := (QUALITY_FACTOR / (QUALITY_FACTOR - MAX_LOSSLESS_SIZE + float32(filelength))) * 100

		if quality = 100; q < quality {
			quality = q
		}

		cfg := webpanimation.NewWebpConfig()
		cfg.SetLossless(0)
		cfg.SetQuality(quality)

		// Append frames
		timeline := 0
		canvas := image.NewRGBA(gif.Image[0].Rect)
		frame := image.NewRGBA(image.Rect(0, 0, int(rw), int(rh)))
		bg := image.NewUniform(image.Transparent)

		var mask draw.Options

		for i, img := range gif.Image {

			mask.SrcMask = img.Bounds()
			mask.SrcMaskP = canvas.Rect.Min

			if gif.Disposal[i] == 3 {
				draw.NearestNeighbor.Scale(frame, frame.Rect, canvas, canvas.Bounds(), draw.Src, nil)
				draw.NearestNeighbor.Scale(frame, frame.Rect, img, img.Bounds(), draw.Over, &mask)
			} else {
				draw.Draw(canvas, canvas.Rect, img, gif.Image[0].Rect.Min, draw.Over)
			}

			draw.NearestNeighbor.Scale(frame, frame.Rect, canvas, canvas.Bounds(), draw.Src, nil)

			if gif.Disposal[i] == 2 {
				draw.NearestNeighbor.Scale(canvas, canvas.Rect, bg, canvas.Rect, draw.Src, &mask)
			}

			if err = anim.AddFrame(frame, timeline, cfg); err != nil {
				log.WithError(err).Error("EditProfilePicture, webp, AddFrame")
				return restutil.ErrInternalServer().Send(c)
			}

			timeline += gif.Delay[i] * 10
		}

		if err = anim.AddFrame(nil, timeline, cfg); err != nil {
			log.WithError(err).Error("EditProfilePicture, webp, AddFrame")
			return restutil.ErrInternalServer().Send(c)
		}

		// Result
		var b bytes.Buffer
		if err = anim.Encode(&b); err != nil {
			return restutil.ErrInternalServer().Send(c, "Encoding Failure")
		}

		// Upload to S3
		id := uuid.New()
		idb, _ := id.MarshalBinary()
		strId := hex.EncodeToString(idb)
		if err = aws.UploadFile(
			configure.Config.GetString("aws_cdn_bucket"),
			fmt.Sprintf("pp/%s/%s", user.ID.Hex(), strId),
			b.Bytes(),
			utils.StringPointer("image/webp"),
		); err != nil {
			log.WithError(err).Error("aws")
			return restutil.ErrInternalServer().Send(c)
		}

		// Update database entry for user
		if _, err = mongo.Collection(mongo.CollectionNameUsers).UpdateByID(ctx, user.ID, bson.M{"$set": bson.M{
			"profile_picture_id": strId,
		}}); err != nil {
			log.WithError(err).Error("mongo")
			return restutil.ErrInternalServer().Send(c)
		}

		user.ProfilePictureID = strId
		j, _ := json.Marshal(EditProfilePictureResult{
			ID:  strId,
			URL: datastructure.UserUtil.GetProfilePictureURL(user),
		})
		return c.Send(j)
	})
}

type EditProfilePictureResult struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}
