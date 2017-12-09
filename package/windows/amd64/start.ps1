
$METADATA_SERVER=IF("$env:METADATA_SERVER" -eq ""){"169.254.169.250"} else {"$env:METADATA_SERVER"}
$RANCHER_METADATA_ANSWER=IF("$env:RANCHER_METADATA_ANSWER" -eq ""){"169.254.169.250"} else {"$env:RANCHER_METADATA_ANSWER"}
$NEVER_RECURSE_TO=IF("$env:NEVER_RECURSE_TO" -eq ""){"169.254.169.251"} else {"$env:NEVER_RECURSE_TO"}
$AGENT_IP=""
function Load-AgentIp {
    while (("$AGENT_IP" -eq "") -or ("$AGENT_IP" -eq "Not found")) {
        $resp=Invoke-WebRequest -UseBasicParsing -Uri "$METADATA_SERVER/2016-07-29/self/host/agent_ip" -ErrorAction Continue -ErrorVariable err
        if("$err" -ne ""){
            Write-Error "Connect to $METADATA_SERVER error, waitting"
            sleep 1
            Continue
        }
        $AGENT_IP=$resp.Content
    }
}

Get-NetAdapter | New-NetIPAddress -IPAddress 169.254.169.251 -PrefixLength 32
Start-Sleep 5
if ( "$RANCHER_METADATA_ANSWER" -eq "agent_ip" ){
    Load-AgentIp 
    $RANCHER_METADATA_ANSWER=$AGENT_IP
}

if ( "$NEVER_RECURSE_TO" -eq "agent_ip" ){
    Load-AgentIp
    $NEVER_RECURSE_TO=$AGENT_IP
}

$cmds =$args + @("-rancher-metadata-answer=$RANCHER_METADATA_ANSWER", "-never-recurse-to=$NEVER_RECURSE_TO")
$cmd= $($cmds -join " ")
Invoke-Expression -Command $cmd