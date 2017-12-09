FROM microsoft/nanoserver
COPY rancher-dns.exe start.ps1 /
ENTRYPOINT [ "powershell","-file","c:/start.ps1" ]
CMD ["C:/rancher-dns.exe"]
