# Go Real-Time Chat Room

A lightweight real-time chat application built with Go and WebSocket. Users can join a persistent chatroom and exchange messages in real-time. The project emphasizes Go's concurrency features with goroutines and channels for handling multiple simultaneous connections.

## Features
- Real-time messaging using WebSocket
- Multiple concurrent user support
- System notifications for user join/leave events
- Simple HTML/JS frontend included

## Quick Start
```bash
# Install dependencies
go get github.com/gorilla/websocket

# Run the server
go run main.go

# Open chat.html in your browser
```