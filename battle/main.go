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

	"github.com/livekit/server-sdk-go/v2/pkg/samplebuilder"
	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4"
)

var botRoomA *lksdk.Room
var botRoomB *lksdk.Room

const (
	maxVideoLate = 100 // nearly 0.2s for fhd video
	maxAudioLate = 5  // 0.1s for audio
	roomNameA    = "A"
	roomNameB    = "C"
)

type TrackAcceptor struct {
	sb         *samplebuilder.SampleBuilder
	track      *webrtc.TrackRemote
	localTrack *lksdk.LocalTrack
}

func (t *TrackAcceptor) start(mimeType, roomName string) {
	for {
		pkt, _, err := t.track.ReadRTP()
		if err != nil {
			break
		}
		t.sb.Push(pkt)

		framePackets := t.sb.PopPackets()
		if len(framePackets) == 0 {
		fmt.Printf("room: %v, mime: %v, No frames popped yet, waiting for more packets...\n", roomName, mimeType)
		} else {
		fmt.Printf("room: %v, mime: %v, Popped %d packets for a complete frame\n", roomName, mimeType, len(framePackets))
		}

		for _, p := range framePackets {
			t.localTrack.WriteRTP(p, nil)
		}
		// mediaFrame := t.sb.Pop()
		// if mediaFrame == nil {
		// 	fmt.Printf("room: %v, mime: %v, No frames popped yet, waiting...\n", roomName, mimeType)
		// 	continue
		// }
		// t.localTrack.WriteSample(*mediaFrame, nil)
	}
}

func NewTrackAcceptor(track *webrtc.TrackRemote, pliWriter lksdk.PLIWriter, destRoom *lksdk.Room) (*TrackAcceptor, error) {
	var sb *samplebuilder.SampleBuilder
	fmt.Printf("track Id: %v, payloadType: %v, kind: %v, codec: %v\n", track.ID(), track.PayloadType(), track.Kind(), track.Codec())

	switch {
	case strings.EqualFold(track.Codec().MimeType, "video/vp8"):
		sb = samplebuilder.New(maxVideoLate, &codecs.VP8Packet{}, track.Codec().ClockRate, samplebuilder.WithPacketDroppedHandler(func() {
			// request a new keyframe to resume playback after packet loss
			pliWriter(track.SSRC())
		}))

	case strings.EqualFold(track.Codec().MimeType, "video/h264"):
		sb = samplebuilder.New(maxVideoLate, &codecs.H264Packet{}, track.Codec().ClockRate, samplebuilder.WithPacketDroppedHandler(func() {
			// request a new keyframe to resume playback after packet loss
			pliWriter(track.SSRC())
		}))

	case strings.EqualFold(track.Codec().MimeType, "audio/opus"):
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
	go t.start(track.Codec().MimeType, destRoom.Name())

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
	fmt.Printf("Connected to room %s\n", room.Name())

	if room.Name() == roomNameA {
		botRoomA = room
	} else {
		botRoomB = room
	}

	<-ctx.Done()
	room.Disconnect()
	fmt.Println("Disconnected from", room.Name())
}

func main() {
	// local deployment
	// host := "ws://localhost:7880"
	// apiKey := "devkey"
	// apiSecret := "secret"
	host := "wss://moj-livestreaming-service.staging.sharechat.com"
	tokenA := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3MzgxNzcxOTMsImlzcyI6IkFQSUxOSmh4RnZqZFVuNCIsIm5iZiI6MTczODE3MzU5Mywic3ViIjoiMTAwMiIsInZpZGVvIjp7InJvb20iOiJBIiwicm9vbUpvaW4iOnRydWV9fQ.B6zwpTcMlH6ID6Dg7c0fWkoa8NyFVguBA54e0UmGL-E"
	tokenB := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3MzgxNzcxOTgsImlzcyI6IkFQSUxOSmh4RnZqZFVuNCIsIm5iZiI6MTczODE3MzU5OCwic3ViIjoiMTAwMiIsInZpZGVvIjp7InJvb20iOiJDIiwicm9vbUpvaW4iOnRydWV9fQ.lpfV02sqdR5HvyKZxCC3NcSkyobbndag6G3oQ1AuXHk"
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
