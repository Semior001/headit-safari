const Header = class {
    constructor(updateDataCallback, key, value, enabled) {
        this.updateData = updateDataCallback;
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

document.addEventListener("DOMContentLoaded", () => {
    let headers = [];
    const container = document.getElementById('headersList');

    let render = () => {};
    let update = () => {
        localStorage.setItem('headers', JSON.stringify(headers));
    };

    let defaultChecked = (setv) => {
        if (setv != null) {
            localStorage.setItem('defaultChecked', JSON.stringify(setv));
        }

        let defaultChecked = false;
        const defCheckedStr = localStorage.getItem('defaultChecked');
        if (defCheckedStr != null && defCheckedStr !== '') {
            defaultChecked = JSON.parse(defCheckedStr);
        }
        return defaultChecked
    }

    render = () => {
        container.innerHTML = '';
        headers.forEach((header, idx) => {
            const btn = document.createElement('button');
            btn.textContent = 'Remove';
            btn.addEventListener('click', () => {
                console.log("[DEBUG] deleting header", idx, header);
                headers.splice(idx, 1);
                render(); update();
            })

            const row = header.row();
            const cell = document.createElement('td');
            cell.appendChild(btn);
            row.appendChild(cell);

            container.appendChild(row);
        });
    }

    document.getElementById('addRow').addEventListener('click', () => {
        console.log("[DEBUG] adding element", headers.length)
        headers.push(new Header(update, '', '', defaultChecked(null)));
        render(); update();
    });

    const result = localStorage.getItem('headers');
    if (result != null && result !== '') {
        const savedHeaders = JSON.parse(result);
        headers = savedHeaders.map(h => new Header(update, h.key, h.value, h.enabled));
    }
    console.log("[DEBUG] loaded headers from local storage:", headers);
    render();

    const toggleAll = document.getElementById('toggleAll');
    toggleAll.addEventListener('change', (e) => {
        console.log("[DEBUG] toggle all", e.target.checked);
        for (let i = 0; i < headers.length; i++) {
            headers[i].enabled = e.target.checked;
        }
        render(); update();
        defaultChecked(e.target.checked);
    });
    toggleAll.checked = defaultChecked(null);
})