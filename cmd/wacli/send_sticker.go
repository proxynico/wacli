package main

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/steipete/wacli/internal/app"
	"github.com/steipete/wacli/internal/store"
	"github.com/steipete/wacli/internal/wa"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

const sendStickerMIME = "image/webp"

type sendStickerOptions struct {
	replyTo       string
	replyToSender string
}

func sendSticker(ctx context.Context, a interface {
	WA() app.WAClient
	DB() *store.DB
}, to types.JID, filePath string, opts sendStickerOptions) (string, map[string]string, error) {
	data, err := readSendFileData(filePath)
	if err != nil {
		return "", nil, err
	}
	if !isWebPStickerData(data) {
		return "", nil, fmt.Errorf("stickers must be WebP files")
	}

	uploadType, err := wa.MediaTypeFromString("sticker")
	if err != nil {
		return "", nil, err
	}
	up, err := a.WA().Upload(ctx, data, uploadType)
	if err != nil {
		return "", nil, err
	}

	replyContext, err := buildReplyContextInfo(a.DB(), to, opts.replyTo, opts.replyToSender)
	if err != nil {
		return "", nil, err
	}
	msg := newStickerMessage(up, replyContext)

	id, err := a.WA().SendProtoMessage(ctx, to, msg)
	if err != nil {
		return "", nil, err
	}

	now := time.Now().UTC()
	name := filepath.Base(filePath)
	chatName := a.WA().ResolveChatName(ctx, to, "")
	_ = a.DB().UpsertChat(to.String(), chatKindFromJID(to), chatName, now)
	_ = a.DB().UpsertMessage(store.UpsertMessageParams{
		ChatJID:       to.String(),
		ChatName:      chatName,
		MsgID:         id,
		SenderJID:     "",
		SenderName:    "me",
		Timestamp:     now,
		FromMe:        true,
		MediaType:     "sticker",
		Filename:      name,
		MimeType:      sendStickerMIME,
		DirectPath:    up.DirectPath,
		MediaKey:      up.MediaKey,
		FileSHA256:    up.FileSHA256,
		FileEncSHA256: up.FileEncSHA256,
		FileLength:    up.FileLength,
	})

	return id, map[string]string{
		"name":      name,
		"mime_type": sendStickerMIME,
		"media":     "sticker",
	}, nil
}

func newStickerMessage(up whatsmeow.UploadResponse, info *waProto.ContextInfo) *waProto.Message {
	return &waProto.Message{
		StickerMessage: &waProto.StickerMessage{
			URL:           proto.String(up.URL),
			DirectPath:    proto.String(up.DirectPath),
			MediaKey:      up.MediaKey,
			FileEncSHA256: up.FileEncSHA256,
			FileSHA256:    up.FileSHA256,
			FileLength:    proto.Uint64(up.FileLength),
			Mimetype:      proto.String(sendStickerMIME),
			ContextInfo:   info,
		},
	}
}

func isWebPStickerData(data []byte) bool {
	return len(data) >= 12 &&
		bytes.Equal(data[0:4], []byte("RIFF")) &&
		bytes.Equal(data[8:12], []byte("WEBP"))
}
