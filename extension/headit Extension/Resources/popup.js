function getAPIBaseURL() {
    let baseURL = localStorage.getItem('apiBaseURL');
    // if empty or not a valid URL, return default
    if (baseURL === null || baseURL === '' || !baseURL.startsWith('http')) {
        return `http://localhost:9096`;
    }
    return baseURL;
}

function loadRules() {
    let rules = localStorage.getItem('rules');
    if (rules === null) {
        return [];
    }
    return JSON.parse(rules).map((rule) => {
        return {
            host:    rule.host,
            key:     rule.key,
            value:   rule.value,
            enabled: rule.enabled
        }
    });
}

document.addEventListener("DOMContentLoaded", () => {
    let currentPageHost = '';

    // noinspection JSUnresolvedReference,JSIgnoredPromiseFromCall
    browser.tabs.query({active: true, currentWindow: true}, (tabs) => {
        currentPageHost = new URL(tabs[0].url).hostname;
        console.log("[DEBUG] current page host", currentPageHost);
        render();
    });

    const headersElem = document.getElementById('headers'); // textarea
    const apiBaseURLElem = document.getElementById('apiBaseURL');       // input

    let render = () => {
        let text = '';
        let rules = loadRules();
        rules.forEach((rule, idx) => {
            if (rule.host !== currentPageHost) {
                return;
            }

            console.log("[DEBUG] rendering rule", idx, rule);

            // add plain text "key: value", if not enabled - add '#' prefix
            text += `${rule.enabled ? '' : '#'}${rule.key}: ${rule.value}\n`;
        });
        headersElem.value = text;
    }

    let update = (rules) => {
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

        console.log("[DEBUG] sending request", req);
        fetch(`${getAPIBaseURL()}/rules`, {
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

    let timeout = null;
    headersElem.addEventListener('input', (e) => {
        console.log("[DEBUG] input event, current timeout", timeout);
        clearTimeout(timeout);
        // do not update immediately, wait for user to finish typing
        timeout = setTimeout(() => {
            console.log("[DEBUG] headers change", e.target.value);

            let lines = e.target.value.split('\n');
            let rules = lines.map((line) => {
                if (line.trim() === '') {
                    return null;
                }

                let tokens = line.split(':');
                if (tokens.length < 2) {
                    return null;
                }

                let header = {
                    host:    currentPageHost,
                    key:     tokens[0].trim(),
                    value:   tokens[1].trim(),
                    enabled: true
                };

                if (line.startsWith('#')) {
                    header.enabled = false;
                    header.key = header.key.substring(1);
                }

                return header;
            }).filter((rule) => rule !== null);

            update(rules);
        }, 500);
    });

    apiBaseURLElem.addEventListener('input', (e) => {
        console.log("[DEBUG] port change", e.target.value);
        localStorage.setItem('apiBaseURL', e.target.value);
    });

    // setup
    apiBaseURLElem.value = getAPIBaseURL();
    console.log("[DEBUG] api server base URL:", getAPIBaseURL());
})
