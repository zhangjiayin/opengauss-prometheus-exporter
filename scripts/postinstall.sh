systemctl daemon-reload >/dev/null 2>&1
systemctl start opengauss_exporter >/dev/null 2>&1
systemctl enable opengauss_exporter >/dev/null 2>&1