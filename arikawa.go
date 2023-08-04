// Package arikawa contains a set of modular packages that allows you to make a
// Discord bot or any type of session (OAuth unsupported).
//
// Session
//
// Package session is the most simple abstraction, which combines the API
// package and the Gateway websocket package together into one. This could be
// used for minimal bots that only use gateway events and such.
//
// State
//
// Package state abstracts on top of session and provides a local cache of API
// calls and events. Bots that either don't need a command router or already has
// its own should use this package.
//
// Bot
//
// Package bot abstracts on top of state and provides a command router based on
// Go code. This is similar to discord.py's API, only it's Go and there's no
// optional arguments (yet, although it could be worked around). Most bots are
// recommended to use this package, as it's the easiest way to make a bot.
//
// Voice
//
// Package voice provides an abstraction on top of State and adds voice support.
// This allows bots to join voice channels and talk. The package uses an
// io.Writer approach rather than a channel, contrary to other Discord
// libraries.
package arikawa

import (
	// Packages that most should use.
	_ "libdb.so/arikawa/v4/session"
	_ "libdb.so/arikawa/v4/state"
	_ "libdb.so/arikawa/v4/voice"

	// Low level packages.
	_ "libdb.so/arikawa/v4/api"
	_ "libdb.so/arikawa/v4/gateway"
)
