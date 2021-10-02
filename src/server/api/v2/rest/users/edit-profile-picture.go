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

func EditProfilePicture(router fiber.Router) {
	router.Patch("/profile-picture", middleware.UserAuthMiddleware(true), func(c *fiber.Ctx) error {
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

		// Read file
		var file *bytes.Reader
		mr := multipart.NewReader(stream, utils.B2S(req.Header.MultipartFormBoundary()))
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
			if len(b) > 1000000 {
				return restutil.ErrBadRequest().Send(c, "Input File Too Large. Must be <1MB")
			}

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

		cfg := webpanimation.NewWebpConfig()
		cfg.SetLossless(0)
		cfg.SetQuality(85)

		// Append frames
		timeline := 0
		for i, img := range gif.Image {
			r := image.NewRGBA(image.Rect(0, 0, int(rw), int(rh)))
			draw.NearestNeighbor.Scale(r, r.Rect, img, img.Bounds(), draw.Src, nil)

			if err = anim.AddFrame(r, timeline, cfg); err != nil {
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
