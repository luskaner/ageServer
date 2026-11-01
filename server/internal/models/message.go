package models

import (
	i "github.com/luskaner/ageServer/server/internal"
)

type Message interface {
	GetTime() int64
	GetBroadcast() bool
	GetContent() string
	GetMessageType() uint8
	GetSender() User
	GetReceivers() []User
	GetAdvertisementId() int32
	GetMetadata() string
	Encode(clientLibVersion uint16) i.A
}

type MainMessage struct {
	advertisementId int32
	time            int64
	broadcast       bool
	content         string
	// Only used for clientLibVersion >= V194, json encoded
	metadata    string
	messageType uint8
	sender      User
	receivers   []User
}

func (message *MainMessage) GetTime() int64 {
	return message.time
}

func (message *MainMessage) GetBroadcast() bool {
	return message.broadcast
}

func (message *MainMessage) GetContent() string {
	return message.content
}

func (message *MainMessage) GetMessageType() uint8 {
	return message.messageType
}

func (message *MainMessage) GetSender() User {
	return message.sender
}

func (message *MainMessage) GetReceivers() []User {
	return message.receivers
}

func (message *MainMessage) GetAdvertisementId() int32 {
	return message.advertisementId
}

func (message *MainMessage) GetMetadata() string {
	return message.metadata
}

func (message *MainMessage) Encode(clientLibVersion uint16) i.A {
	msg := i.A{
		message.sender.GetId(),  // senderID
		message.content,         // filteredMessage
		message.content,         // content
		message.messageType,     // messageType
		message.advertisementId, // sessionID
	}
	if clientLibVersion >= i.V194 {
		msg = append(msg, message.metadata) // metadata
	}
	return msg
}
