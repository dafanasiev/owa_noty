#!/usr/bin/env bash
sudo cp owa_noty.service /usr/lib/systemd/user/
sudo chown root:root /usr/lib/systemd/user/owa_noty.service
sudo chmod 644 /usr/lib/systemd/user/owa_noty.service

sudo rm -rf /opt/owa_noty
sudo mkdir /opt/owa_noty
sudo cp ../bin/owa_noty /opt/owa_noty/
sudo cp ../bin/newmail.png /opt/owa_noty/

rm -rf ~/.config/owa_noty
mkdir ~/.config/owa_noty
cp ../bin/config.json ~/.config/owa_noty/

systemctl --user enable owa_noty
systemctl --user start owa_noty

