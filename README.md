# DAMT ![Build](https://github.com/someone-stole-my-name/DAMT/workflows/Build/badge.svg)
Delete your tweets before a specific date

### Usage:
```
-config string
        JSON Config File
  -days int
        Tweets older than this value will be deleted
  -help
        Prints this message
  -test
        Do not delete anything
        
   Example:
   Delete all tweets older than 7 days
   ./DAMT --config configfile.json --days 7
   
   Delete all my tweets
   ./DAMT --config configfile.json
```

#### ConfigFile
```
{
    "APIKey":"xxxx",
    "APISecretKey":"xxxx",
    "AccessToken":"xxxx",
    "AccessTokenSecret":"xxxx"
}
```
