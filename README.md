Simple discord bot that automatically replies to some common keywords in questions.

Usage of M45HelpBot:

  -guildid string
  
        discord guild id
  
  -helpFilePath string
  
        Specify path to helps file. (default "helps.json")
  
  -staffid string

        discord role ID for moderator/staff

  -testmode
 
        skip throttle check

  -token string
  
        discord token

Testing:

  go test ./...

Live link checks are opt-in:

  LINKCHECK=1 go test -run 'TestHelp.*Links' -v

Discord channel and message links require a bot token with access to the
server/channel:

  LINKCHECK=1 LINKCHECK_DISCORD_TOKEN='your-token' go test -run TestHelpDiscordLinksResolve -v
