Name:           node_exporter
Version:        {version}
Release:        0
Summary:        node_exporter build for openEuler-os-infra

Group:          System Environment/Daemons
License:        GPL
Packager:       TomNewChao

%description
This RPM package is based on the node_export package

%prep

%pre
grep node_exporter /etc/passwd > /dev/null
if [ $? != 0 ]
then
  useradd node_exporter -M -s /sbin/nologin
fi
#ss -lntp |grep ":9100" >/dev/null
#if [ $? == 0 ]
#then
#  sed -i 's#ExecStart.*#ExecStart=/usr/bin/node_exporter --web.listen-address=:1900#g' /usr/lib/systemd/system/node_exporter.service
#  systemctl daemon-reload
#fi

%post
chmod +x /usr/bin/node_exporter
systemctl enable node_exporter
systemctl start node_exporter


%preun
systemctl stop node_exporter
systemctl disable node_exporter

%postun
[ -L /etc/systemd/system/multi-user.target.wants/node_exporter.service ] && unlink /etc/systemd/system/multi-user.target.wants/node_exporter.service
rm -rf /usr/bin/node_exporter

%files
%defattr (-,root,root,0777)
/usr/bin/node_exporter
/usr/lib/systemd/system/node_exporter.service