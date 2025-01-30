package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
)

var firstParticipantSubscribed = false

func onTrackSubscribed(track *webrtc.TrackRemote, echoTrack *lksdk.LocalTrack) {
	for {
		pkt, _, err := track.ReadRTP()
		if err != nil {
			continue
		}
		echoTrack.WriteSample(media.Sample{Data: pkt.Payload, Duration: 20 * time.Millisecond}, &lksdk.SampleWriteOptions{})
	}
}

func main() {
	host := "wss://moj-livestreaming-service.staging.sharechat.com"
	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3MzgyMjMxNzQsImlzcyI6IkFQSUxOSmh4RnZqZFVuNCIsIm5iZiI6MTczODIxOTU3NCwic3ViIjoiMTAwMSIsInZpZGVvIjp7InJvb20iOiJBIiwicm9vbUpvaW4iOnRydWV9fQ.mGoXmY49TpORKlX_XZIXMzh3rVKEXSJRS85DtajJCHo"

	echoTrack, err := lksdk.NewLocalTrack(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus})
	if err != nil {
		panic(err)
	}

	room := lksdk.NewRoom(&lksdk.RoomCallback{
		ParticipantCallback: lksdk.ParticipantCallback{
			OnTrackSubscribed: func(track *webrtc.TrackRemote, publication *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
				// Only provide echo for the first participant
				if !firstParticipantSubscribed && track.Kind() == webrtc.RTPCodecTypeAudio {
					firstParticipantSubscribed = true
					onTrackSubscribed(track, echoTrack)
				}
			},
		},
	})

	// not required. warm up the connection for a participant that may join later.
	if err := room.PrepareConnection(host, token); err != nil {
		panic(err)
	}

	if err := room.JoinWithToken(host, token); err != nil {
		panic(err)
	}

	if _, err = room.LocalParticipant.PublishTrack(echoTrack, &lksdk.TrackPublicationOptions{
		Name: "echo",
	}); err != nil {
		panic(err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT)

	<-sigChan
	room.Disconnect()
}
