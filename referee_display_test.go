// Copyright 2014 Team 254. All Rights Reserved.
// Author: pat@patfairbank.com (Patrick Fairbank)

package main

import (
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestRefereeDisplay(t *testing.T) {
	clearDb()
	defer clearDb()
	var err error
	db, err = OpenDatabase(testDbPath)
	assert.Nil(t, err)
	defer db.Close()
	eventSettings, _ = db.GetEventSettings()
	mainArena.Setup()

	recorder := getHttpResponse("/displays/referee")
	assert.Equal(t, 200, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Referee Display - Untitled Event - Cheesy Arena")
}

func TestRefereeDisplayWebsocket(t *testing.T) {
	clearDb()
	defer clearDb()
	var err error
	db, err = OpenDatabase(testDbPath)
	assert.Nil(t, err)
	defer db.Close()
	eventSettings, _ = db.GetEventSettings()
	mainArena.Setup()

	server, wsUrl := startTestServer()
	defer server.Close()
	conn, _, err := websocket.DefaultDialer.Dial(wsUrl+"/displays/referee/websocket", nil)
	assert.Nil(t, err)
	defer conn.Close()
	ws := &Websocket{conn}

	// Test foul addition.
	foulData := struct {
		Alliance       string
		TeamId         int
		Rule           string
		TimeInMatchSec float64
		IsTechnical    bool
	}{"red", 256, "G22", 0, false}
	ws.Write("addFoul", foulData)
	foulData.TeamId = 359
	foulData.IsTechnical = true
	ws.Write("addFoul", foulData)
	foulData.Alliance = "blue"
	foulData.TeamId = 1680
	ws.Write("addFoul", foulData)
	readWebsocketType(t, ws, "reload")
	readWebsocketType(t, ws, "reload")
	readWebsocketType(t, ws, "reload")
	if assert.Equal(t, 2, len(mainArena.redRealtimeScore.Fouls)) {
		assert.Equal(t, 256, mainArena.redRealtimeScore.Fouls[0].TeamId)
		assert.Equal(t, "G22", mainArena.redRealtimeScore.Fouls[0].Rule)
		assert.Equal(t, 0, mainArena.redRealtimeScore.Fouls[0].TimeInMatchSec)
		assert.Equal(t, false, mainArena.redRealtimeScore.Fouls[0].IsTechnical)
		assert.Equal(t, 359, mainArena.redRealtimeScore.Fouls[1].TeamId)
		assert.Equal(t, "G22", mainArena.redRealtimeScore.Fouls[1].Rule)
		assert.Equal(t, true, mainArena.redRealtimeScore.Fouls[1].IsTechnical)
	}
	if assert.Equal(t, 1, len(mainArena.blueRealtimeScore.Fouls)) {
		assert.Equal(t, 1680, mainArena.blueRealtimeScore.Fouls[0].TeamId)
		assert.Equal(t, "G22", mainArena.blueRealtimeScore.Fouls[0].Rule)
		assert.Equal(t, 0, mainArena.blueRealtimeScore.Fouls[0].TimeInMatchSec)
		assert.Equal(t, true, mainArena.blueRealtimeScore.Fouls[0].IsTechnical)
	}
	assert.False(t, mainArena.redRealtimeScore.FoulsCommitted)
	assert.False(t, mainArena.blueRealtimeScore.FoulsCommitted)

	// Test foul deletion.
	ws.Write("deleteFoul", foulData)
	readWebsocketType(t, ws, "reload")
	assert.Equal(t, 0, len(mainArena.blueRealtimeScore.Fouls))
	foulData.Alliance = "red"
	foulData.TeamId = 359
	foulData.TimeInMatchSec = 29 // Make it not match.
	ws.Write("deleteFoul", foulData)
	readWebsocketType(t, ws, "reload")
	assert.Equal(t, 2, len(mainArena.redRealtimeScore.Fouls))
	foulData.TimeInMatchSec = 0
	ws.Write("deleteFoul", foulData)
	readWebsocketType(t, ws, "reload")
	assert.Equal(t, 1, len(mainArena.redRealtimeScore.Fouls))

	// Test card setting.
	cardData := struct {
		Alliance string
		TeamId   int
		Card     string
	}{"red", 256, "yellow"}
	ws.Write("card", cardData)
	cardData.Alliance = "blue"
	cardData.TeamId = 1680
	cardData.Card = "red"
	ws.Write("card", cardData)
	time.Sleep(time.Millisecond * 10) // Allow some time for the command to be processed.
	if assert.Equal(t, 1, len(mainArena.redRealtimeScore.Cards)) {
		assert.Equal(t, "yellow", mainArena.redRealtimeScore.Cards["256"])
	}
	if assert.Equal(t, 1, len(mainArena.blueRealtimeScore.Cards)) {
		assert.Equal(t, "red", mainArena.blueRealtimeScore.Cards["1680"])
	}

	// Test field reset and match committing.
	mainArena.MatchState = POST_MATCH
	ws.Write("signalReset", nil)
	time.Sleep(time.Millisecond * 10)
	assert.True(t, mainArena.redRealtimeScore.FieldReset)
	assert.True(t, mainArena.blueRealtimeScore.FieldReset)
	assert.False(t, mainArena.redRealtimeScore.FoulsCommitted)
	assert.False(t, mainArena.blueRealtimeScore.FoulsCommitted)
	mainArena.redRealtimeScore.FieldReset = false
	mainArena.blueRealtimeScore.FieldReset = false
	ws.Write("commitMatch", nil)
	readWebsocketType(t, ws, "reload")
	assert.True(t, mainArena.redRealtimeScore.FieldReset)
	assert.True(t, mainArena.blueRealtimeScore.FieldReset)
	assert.True(t, mainArena.redRealtimeScore.FoulsCommitted)
	assert.True(t, mainArena.blueRealtimeScore.FoulsCommitted)

	// Should refresh the page when the next match is loaded.
	mainArena.matchLoadTeamsNotifier.Notify(nil)
	readWebsocketType(t, ws, "reload")
}
