package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

func TestSendCommandIncludesStickerSubcommand(t *testing.T) {
	cmd := newSendCmd(&rootFlags{})
	for _, sub := range cmd.Commands() {
		if sub.Name() == "sticker" {
			return
		}
	}
	t.Fatalf("missing send sticker subcommand")
}

func TestSendStickerCommandExposesSharedSendFlags(t *testing.T) {
	cmd := newSendStickerCmd(&rootFlags{})
	for _, name := range []string{"to", "pick", "file", "reply-to", "reply-to-sender", "post-send-wait"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Fatalf("missing --%s flag", name)
		}
	}
}

func TestIsWebPStickerData(t *testing.T) {
	valid := []byte("RIFF\x10\x00\x00\x00WEBPVP8 ")
	if !isWebPStickerData(valid) {
		t.Fatalf("valid WebP header was rejected")
	}
	for _, data := range [][]byte{
		nil,
		[]byte("RIFF\x10\x00\x00\x00PNG "),
		[]byte("not webp"),
	} {
		if isWebPStickerData(data) {
			t.Fatalf("invalid WebP header was accepted: %q", string(data))
		}
	}
}

func TestSendStickerRejectsNonWebPBeforeUpload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sticker.png")
	if err := os.WriteFile(path, []byte("not-webp"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, _, err := sendSticker(context.Background(), nil, types.JID{}, path, sendStickerOptions{})
	if err == nil || !strings.Contains(err.Error(), "stickers must be WebP") {
		t.Fatalf("expected WebP validation error, got %v", err)
	}
}

func TestNewStickerMessageAttachesUploadFieldsAndReply(t *testing.T) {
	up := whatsmeow.UploadResponse{
		URL:           "https://upload",
		DirectPath:    "/direct",
		MediaKey:      []byte("key"),
		FileEncSHA256: []byte("enc"),
		FileSHA256:    []byte("plain"),
		FileLength:    123,
	}
	info := &waProto.ContextInfo{
		StanzaID:    proto.String("quoted"),
		Participant: proto.String("15551234567@s.whatsapp.net"),
	}

	msg := newStickerMessage(up, info)
	sticker := msg.GetStickerMessage()
	if sticker == nil {
		t.Fatalf("missing sticker message")
	}
	if sticker.GetMimetype() != sendStickerMIME {
		t.Fatalf("mime = %q, want %q", sticker.GetMimetype(), sendStickerMIME)
	}
	if sticker.GetURL() != up.URL || sticker.GetDirectPath() != up.DirectPath || sticker.GetFileLength() != up.FileLength {
		t.Fatalf("upload fields were not attached")
	}
	if string(sticker.GetMediaKey()) != string(up.MediaKey) ||
		string(sticker.GetFileSHA256()) != string(up.FileSHA256) ||
		string(sticker.GetFileEncSHA256()) != string(up.FileEncSHA256) {
		t.Fatalf("upload hashes were not attached")
	}
	if sticker.GetContextInfo() != info {
		t.Fatalf("reply context was not attached")
	}
}
