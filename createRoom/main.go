package main

import (
	"context"
	"fmt"

	"createRoom/helpers"

	"github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go/v2"
)

func main() {
	host := "ws://localhost:7880"
	apiKey := "devkey"
	apiSecret := "secret"
	roomName := "roomB"
	roomClient := lksdk.NewRoomServiceClient(host, apiKey, apiSecret)
	_, _ = roomClient.CreateRoom(context.Background(), &livekit.CreateRoomRequest{
		Name:            roomName,
		EmptyTimeout:    60 * 60, // 1 hour
		MaxParticipants: 20,
	})

	// fetch token
	fmt.Println(helpers.GetJoinToken(apiKey, apiSecret, roomName, "1001"))
}

// list rooms
// res, _ := roomClient.ListRooms(context.Background(), &livekit.ListRoomsRequest{})
// rooms := res.Rooms
// for _, room := range rooms {
// 	fmt.Printf("name: %s, numParticipants: %v\n", room.Name, room.NumParticipants)
// }
// // list participants in a room
// p, _ := roomClient.ListParticipants(context.Background(), &livekit.ListParticipantsRequest{
// 	Room: "myroom",
// })
// for _, participant := range p.Participants {
// 	fmt.Printf("name: %s, identity: %s, state: %v\n", participant.Name, participant.Identity, participant.State)
// 	if participant.Identity == "1001" {
// 		tracks := participant.Tracks
// 		for _, track := range tracks {
// 			fmt.Printf("track: %s, sid: %v, type: %v, muted: %v\n", track.Name, track.Sid, track.Type, track.Muted)
// 		}
// 	}
// }
// // mute/unmute participant's tracks
// roomClient.MutePublishedTrack(context.Background(), &livekit.MuteRoomTrackRequest{
// 	Room:     "myroom",
// 	Identity: "1001",
// 	TrackSid: "TR_VCPrheWfWFt6Xx",
// 	Muted:    false,
// })
