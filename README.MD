# SevenTv

1. Install golang
```bash
wget https://golang.org/dl/go1.16.2.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.16.2.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
```

2. Clone the repo
```bash
cd /opt/
git clone https://github.com/SevenTV/ServerGo.git
cd seventv
```

3. Build the source
```bash
go build
```

4. Modify the config.
```bash
cp demo.yaml config.yaml && vim config.yaml
```

6. Setup auto run with systemboot.
```bash
sudo cp seventv.service /etc/systemd/system && sudo systemctl start seventv && sudo systemctl enable seventv
```
