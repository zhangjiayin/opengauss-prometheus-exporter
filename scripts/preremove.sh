systemctl stop opengauss_exporter >/dev/null 2>&1
systemctl disable opengauss_exporter >/dev/null 2>&1
systemctl daemon-reload >/dev/null 2>&1