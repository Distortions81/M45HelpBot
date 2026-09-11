Simple discord bot that automatically replies to some common keywords in questions.

Setup:

  Enable Message Content Intent under Bot > Privileged Gateway Intents in the
  Discord Developer Portal. The bot needs it to read ordinary chat messages.
  Supply both `-token` and `-guildid` when starting the bot.

Usage of M45HelpBot:

  -guildid string
  
        discord guild id
  
  -helpFilePath string
  
        Specify path to helps file. (default "helps.json")
  
  -staffChannel string

        specify a staff-only channel

  -staffid string

        discord role ID for moderator/staff

  -testmode
 
        skip throttle check

  -token string
  
        discord token

Normal replies are limited to one per user every 24 hours and one globally every
15 minutes, with caps of 3 per user and 14 globally per process run. Failed sends
do not consume these allowances. `-testmode` bypasses the limits.

The process exits after 7 days; run it under a service manager configured to
restart it. Shutdown leaves the help file untouched. Restart to load help edits.

Help replies describe ChatWire commands. Keep command names, registration steps,
and role requirements aligned with ChatWire's handlers and the
[M45 player help](https://m45sci.xyz/help-info.html) and
[staff help](https://m45sci.xyz/help-discord-staff.html).

Testing:

  go test ./...

  go test -race ./...

  go vet ./...

Live link checks are opt-in:

  LINKCHECK=1 go test -run 'TestHelp.*Links' -v

Discord channel and message links require a bot token with access to the
server/channel:

  LINKCHECK=1 LINKCHECK_DISCORD_TOKEN='your-token' go test -run TestHelpDiscordLinksResolve -v
