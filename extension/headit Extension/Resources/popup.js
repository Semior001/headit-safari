const Header = class {
    constructor(updateDataCallback, host, key, value, enabled) {
        this.updateData = updateDataCallback;
        this.host = host;
        this.key = key;
        this.value = value;
        this.enabled = enabled;
    }

    row() {
        const row = document.createElement('tr');
        row.appendChild(this.checkboxCell());
        row.appendChild(this.keyCell());
        row.appendChild(this.valueCell());
        return row;
    }

    checkboxCell() {
        const cb = document.createElement('input');
        cb.type = 'checkbox';
        cb.checked = this.enabled;
        cb.addEventListener('change', (e) => {
            console.log("[DEBUG] checked change on checkbox of header", this, e);
            this.enabled = e.target.checked;
            this.updateData();
        });

        const cell = document.createElement('td');
        cell.appendChild(cb);
        return cell;
    }

    keyCell() {
        const inp = document.createElement('input');
        inp.value = this.key;
        inp.addEventListener('input', (e) => {
            this.key = e.target.value;
            this.updateData();
        });

        const cell = document.createElement('td');
        cell.appendChild(inp);
        return cell;
    }

    valueCell() {
        const inp = document.createElement('input');
        inp.value = this.value;
        inp.addEventListener('input', (e) => {
            this.value = e.target.value;
            this.updateData();
        });

        const cell = document.createElement('td');
        cell.appendChild(inp);
        return cell
    }
}

function getPort() {
    let port = localStorage.getItem('port');
    if (port === null || port === '') {
        return '9096';
    }
    return port;
}

function getDefaultChecked() {
    return localStorage.getItem('defaultChecked') === 'true';
}

function loadRules(cb) {
    let rules = localStorage.getItem('rules');
    if (rules === null) {
        return [];
    }
    return JSON.parse(rules).map((rule) => {
        return new Header(cb, rule.host, rule.key, rule.value, rule.enabled);
    });
}

document.addEventListener("DOMContentLoaded", () => {
    let rules = [];
    let currentPageHost = '';
    browser.tabs.query({active: true, currentWindow: true}, (tabs) => {
        currentPageHost = new URL(tabs[0].url).hostname;
        console.log("[DEBUG] current page host", currentPageHost);
        render();
    });

    const containerElem = document.getElementById('headersList');
    const addRowElem = document.getElementById('addRow');
    const portElem = document.getElementById('port');
    const toggleAllElem = document.getElementById('toggleAll');

    let render = () => {
        containerElem.innerHTML = ''
        rules.forEach((rule, idx) => {
            if (rule.host !== currentPageHost) {
                return;
            }

            console.log("[DEBUG] rendering rule", idx, rule);

            const btn = document.createElement('button');
            btn.textContent = 'Remove';
            btn.addEventListener('click', () => {
                console.log("[DEBUG] deleting rule", idx, rule);
                rules.splice(idx, 1);
                render(); update(true);
            })

            const row = rule.row();
            const cell = document.createElement('td');
            cell.appendChild(btn);
            row.appendChild(cell);

            containerElem.appendChild(row);
        });
    }

    let update = (force) => {
        console.log("[DEBUG] updating headers", rules);
        localStorage.setItem('rules', JSON.stringify(rules));

        let hosts = {};
        for (let i = 0; i < rules.length; i++) {
            if (!rules[i].enabled) {
                continue;
            }
            if (hosts[rules[i].host] === undefined) {
                hosts[rules[i].host] = {};
            }
            hosts[rules[i].host][rules[i].key] = rules[i].value;
        }

        let req = [];
        for (let host in hosts) {
            req.push({host: host, add_headers: hosts[host]});
        }

        if (req.length === 0 && !force) {
            return;
        }

        console.log("[DEBUG] sending request", req);
        fetch(`http://localhost:${getPort()}/rules`, {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify(req)
        }).then((response) => {
            response.text().then((data) => {
                console.log("[DEBUG] response", data);
            });
        }).catch((error) => {
            console.log("[ERROR] update error", error);
        });
    };

    addRowElem.addEventListener('click', () => {
        console.log("[DEBUG] adding element", rules.length)
        rules.push(new Header(update, currentPageHost, '', '', getDefaultChecked()));
        render(); update(false);
    });

    portElem.addEventListener('input', (e) => {
        console.log("[DEBUG] port change", e.target.value);
        localStorage.setItem('port', e.target.value);
    });

    toggleAllElem.addEventListener('change', (e) => {
        console.log("[DEBUG] toggle all", e.target.checked);
        for (let i = 0; i < rules.length; i++) {
            rules[i].enabled = e.target.checked;
        }
        render(); update(true);
        localStorage.setItem('defaultChecked', e.target.checked);
    });

    // setup
    portElem.value = getPort();
    toggleAllElem.checked = getDefaultChecked();
    rules = loadRules(update);

    console.log("[DEBUG] api server port: ", getPort());
    console.log("[DEBUG] check by default: ", getDefaultChecked());
    console.log("[DEBUG] headers: ", rules);
})
