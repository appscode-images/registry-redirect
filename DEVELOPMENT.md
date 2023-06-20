# Development Notes

```bash
# on development machine
make build OS=linux ARCH=amd64
scp bin/registry-redirect-linux-amd64 root@r.appscode.com:/root


# on production server
> ssh root@r.appscode.com

chmod +x registry-redirect-linux-amd64
mv registry-redirect-linux-amd64 /usr/local/bin/registry-redirect
sudo systemctl restart registry-redirect
```
