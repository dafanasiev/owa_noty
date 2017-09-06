sudo cp owa_noty.service /usr/lib/systemd/user/
sudo chown root:root /usr/lib/systemd/user/owa_noty.service
sudo chmod 644 /usr/lib/systemd/user/owa_noty.service


sudo rm -rf /opt/owa_noty
sudo mkdir /opt/owa_noty
sudo cp ../bin/* /opt/owa_noty/

systemctl --user enable owa_noty
systemctl --user start owa_noty

