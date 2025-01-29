package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/livekit/server-sdk-go/pkg/samplebuilder"
	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4"
)

var botRoomA *lksdk.Room
var botRoomB *lksdk.Room

const (
	maxVideoLate = 1000 // nearly 2s for fhd video
	maxAudioLate = 200  // 4s for audio
	roomNameA    = "A"
	roomNameB    = "C"
)

type TrackAcceptor struct {
	sb         *samplebuilder.SampleBuilder
	track      *webrtc.TrackRemote
	localTrack *lksdk.LocalTrack
}

func (t *TrackAcceptor) start() {
	for {
		pkt, _, err := t.track.ReadRTP()
		if err != nil {
			break
		}
		t.sb.Push(pkt)

		for _, p := range t.sb.PopPackets() {
			t.localTrack.WriteRTP(p, nil)
		}
	}
}

func NewTrackAcceptor(track *webrtc.TrackRemote, pliWriter lksdk.PLIWriter, destRoom *lksdk.Room) (*TrackAcceptor, error) {
	var (
		sb *samplebuilder.SampleBuilder
	)

	switch {
	case strings.EqualFold(track.Codec().MimeType, "video/vp8"):
		fmt.Println("codec: vp8")
		sb = samplebuilder.New(maxVideoLate, &codecs.VP8Packet{}, track.Codec().ClockRate, samplebuilder.WithPacketDroppedHandler(func() {
			pliWriter(track.SSRC())
		}))

	case strings.EqualFold(track.Codec().MimeType, "video/h264"):
		fmt.Println("codec: h264")
		sb = samplebuilder.New(maxVideoLate, &codecs.H264Packet{}, track.Codec().ClockRate, samplebuilder.WithPacketDroppedHandler(func() {
			pliWriter(track.SSRC())
		}))

	case strings.EqualFold(track.Codec().MimeType, "audio/opus"):
		fmt.Println("codec: opus")
		sb = samplebuilder.New(maxAudioLate, &codecs.OpusPacket{}, track.Codec().ClockRate)
	default:
		return nil, errors.New("unsupported codec type")
	}

	// Create a LocalTrack for Room
	localTrack, err := lksdk.NewLocalTrack(track.Codec().RTPCodecCapability)
	if err != nil {
		return nil, err
	}

	// Publish the local track to Room
	_, err = destRoom.LocalParticipant.PublishTrack(localTrack, &lksdk.TrackPublicationOptions{})
	if err != nil {
		return nil, err
	}

	t := &TrackAcceptor{
		sb:         sb,
		track:      track,
		localTrack: localTrack,
	}
	go t.start()

	return t, nil
}

func trackSubscribedB(track *webrtc.TrackRemote, publication *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
	NewTrackAcceptor(track, rp.WritePLI, botRoomA)
}

func trackSubscribedA(track *webrtc.TrackRemote, publication *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
	NewTrackAcceptor(track, rp.WritePLI, botRoomB)
}

func relayRoom(wg *sync.WaitGroup, ctx context.Context, host, roomName, token string) {
	defer wg.Done()

	//room callback
	roomCB := &lksdk.RoomCallback{
		ParticipantCallback: lksdk.ParticipantCallback{
			OnTrackSubscribed: trackSubscribedA,
		},
	}

	if roomName == roomNameB {
		roomCB.ParticipantCallback.OnTrackSubscribed = trackSubscribedB
	}

	// room, err := lksdk.ConnectToRoom(host, lksdk.ConnectInfo{
	// 	APIKey:              apiKey,
	// 	APISecret:           apiSecret,
	// 	RoomName:            roomName,
	// 	ParticipantIdentity: "1002",
	// }, roomCB)

	room, err := lksdk.ConnectToRoomWithToken(host, token, roomCB)
	if err != nil {
		fmt.Printf("Failed to connect to room %s due to error: %s", roomName, err.Error())
		return
	}
	fmt.Printf("Connected to room %s\n", roomName)

	if roomName == roomNameA {
		botRoomA = room
	} else {
		botRoomB = room
	}

	<-ctx.Done()
	room.Disconnect()
	fmt.Println("Disconnected from", roomName)
}

func main() {
	// local deployment
	// host := "ws://localhost:7880"
	// apiKey := "devkey"
	// apiSecret := "secret"
	host := "wss://moj-livestreaming-service.staging.sharechat.com"
	tokenA := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3MzgxNDQ2OTAsImlzcyI6IkFQSUxOSmh4RnZqZFVuNCIsIm5iZiI6MTczODE0MTA5MCwic3ViIjoiMTAwMiIsInZpZGVvIjp7InJvb20iOiJBIiwicm9vbUpvaW4iOnRydWV9fQ.f0dwIr5nWHtjzU9cQMO1M63iO0WYPMYXCg8ARkQeDgQ"
	tokenB := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3MzgxNDQ2OTUsImlzcyI6IkFQSUxOSmh4RnZqZFVuNCIsIm5iZiI6MTczODE0MTA5NSwic3ViIjoiMTAwMiIsInZpZGVvIjp7InJvb20iOiJDIiwicm9vbUpvaW4iOnRydWV9fQ.R98oZgUu0-WB3LD8OufR1duOCwpUT0iGAtfuiiNaTak"
	// roomClient := lksdk.NewRoomServiceClient(host, apiKey, apiSecret)

	// // list rooms
	// res, _ := roomClient.ListRooms(context.Background(), &livekit.ListRoomsRequest{})
	// rooms := res.Rooms
	// for _, room := range rooms {
	// 	fmt.Printf("name: %s, numParticipants: %v\n", room.Name, room.NumParticipants)
	// }

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)
	go relayRoom(&wg, ctx, host, roomNameA, tokenA)
	go relayRoom(&wg, ctx, host, roomNameB, tokenB)

	<-sigChan
	fmt.Println("\nReceived shutdown signal. Disconnecting...")
	// Cancel context to notify both goroutines
	cancel()
	wg.Wait()
	fmt.Println("Graceful shutdown completed.")
}
