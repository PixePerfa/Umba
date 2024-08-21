const instanceList = document.getElementById('instance-list');
const createInstanceBtn = document.getElementById('create-instance-btn');

async function loadInstances() {
    try {
        const response = await fetch('/api/v1/instances');
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        const instances = await response.json();
        instanceList.innerHTML = '';
        instances.forEach(instance => {
            const row = document.createElement('tr');
            row.innerHTML = `
                <td>${instance.ID}</td>
                <td>${instance.Status}</td>
                <td>${instance.URL}</td>
                <td>
                    <button class="control-btn" data-id="${instance.ID}">Control</button>
                    <button class="debug-btn" data-id="${instance.ID}">Debug</button>
                    <button class="delete-btn" data-id="${instance.ID}">Delete</button>
                </td>
            `;
            instanceList.appendChild(row);
        });
        document.querySelectorAll('.control-btn').forEach(btn => {
            btn.addEventListener('click', () => openControlDialog(btn.getAttribute('data-id')));
        });
        document.querySelectorAll('.debug-btn').forEach(btn => {
            btn.addEventListener('click', () => openDebugDialog(btn.getAttribute('data-id')));
        });
        document.querySelectorAll('.delete-btn').forEach(btn => {
            btn.addEventListener('click', () => deleteInstance(btn.getAttribute('data-id')));
        });
    } catch (error) {
        console.error('Error loading instances:', error);
        alert('Failed to load instances. Please try again later.');
    }
}

async function createInstance(url) {
    try {
        const response = await fetch('/api/v1/instances', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ URL: url })
        });
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        await loadInstances();
        showMainDashboard();
    } catch (error) {
        console.error('Error creating instance:', error);
        alert('Failed to create instance. Please try again.');
    }
}

async function deleteInstance(instanceID) {
    try {
        const response = await fetch(`/api/v1/instances/${instanceID}`, {
            method: 'DELETE'
        });
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        await loadInstances();
    } catch (error) {
        console.error('Error deleting instance:', error);
        alert('Failed to delete instance. Please try again.');
    }
}

function openControlDialog(instanceID) {
    document.getElementById('control-instance-section').style.display = 'block';
    document.getElementById('create-instance-section').style.display = 'none';
    document.getElementById('debug-instance-section').style.display = 'none';
    document.getElementById('flow-manager-section').style.display = 'none';
    document.getElementById('start-stop-btn').addEventListener('click', async () => {
        const status = document.getElementById('start-stop-btn').textContent === 'Start' ? 'running' : 'stopped';
        try {
            const response = await fetch(`/api/v1/instances/${instanceID}/status`, {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({ Status: status })
            });
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            document.getElementById('start-stop-btn').textContent = status === 'running' ? 'Stop' : 'Start';
        } catch (error) {
            console.error('Error updating instance status:', error);
            alert('Failed to update instance status. Please try again.');
        }
    });
}

function openDebugDialog(instanceID) {
    document.getElementById('debug-instance-section').style.display = 'block';
    document.getElementById('create-instance-section').style.display = 'none';
    document.getElementById('control-instance-section').style.display = 'none';
    document.getElementById('flow-manager-section').style.display = 'none';
    loadScreenshot(instanceID);
    document.getElementById('refresh-screenshot-btn').addEventListener('click', () => loadScreenshot(instanceID));
}

async function loadScreenshot(instanceID) {
    try {
        const response = await fetch(`/api/v1/instances/${instanceID}/screenshot`);
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        const blob = await response.blob();
        const screenshot = document.getElementById('screenshot');
        screenshot.src = URL.createObjectURL(blob);
    } catch (error) {
        console.error('Error loading screenshot:', error);
        alert('Failed to load screenshot. Please try again.');
    }
}

function showMainDashboard() {
    document.getElementById('create-instance-section').style.display = 'none';
    document.getElementById('control-instance-section').style.display = 'none';
    document.getElementById('debug-instance-section').style.display = 'none';
    document.getElementById('flow-manager-section').style.display = 'none';
    loadInstances();
}

createInstanceBtn.addEventListener('click', () => {
    document.getElementById('create-instance-section').style.display = 'block';
    document.getElementById('control-instance-section').style.display = 'none';
    document.getElementById('debug-instance-section').style.display = 'none';
    document.getElementById('flow-manager-section').style.display = 'none';
    document.getElementById('create-instance-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const url = document.getElementById('url').value;
        if (url) {
            await createInstance(url);
        }
    });
    document.getElementById('launch-url-btn').addEventListener('click', () => {
        const url = document.getElementById('url').value;
        if (url) {
            window.open(url, '_blank');
        }
    });
});

document.getElementById('create-flow-btn').addEventListener('click', async () => {
    const name = prompt('Enter flow name:');
    if (name) {
        try {
            const response = await fetch('/api/v1/flows', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({ Name: name })
            });
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            await loadFlows();
        } catch (error) {
            console.error('Error creating flow:', error);
            alert('Failed to create flow. Please try again.');
        }
    }
});

async function loadFlows() {
    try {
        const response = await fetch('/api/v1/flows');
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        const flows = await response.json();
        const flowList = document.getElementById('flow-list');
        flowList.innerHTML = '';
        flows.forEach(flow => {
            const div = document.createElement('div');
            div.innerHTML = `
                <h2>${flow.Name}</h2>
                <button class="execute-flow-btn" data-id="${flow.ID}">Execute Flow</button>
                <button class="delete-flow-btn" data-id="${flow.ID}">Delete Flow</button>
            `;
            flowList.appendChild(div);
        });
        document.querySelectorAll('.execute-flow-btn').forEach(btn => {
            btn.addEventListener('click', async () => {
                const flowID = btn.getAttribute('data-id');
                try {
                    const response = await fetch(`/api/v1/flows/${flowID}/execute`, {
                        method: 'POST'
                    });
                    if (!response.ok) {
                        throw new Error(`HTTP error! status: ${response.status}`);
                    }
                    alert('Flow executed');
                } catch (error) {
                    console.error('Error executing flow:', error);
                    alert('Failed to execute flow. Please try again.');
                }
            });
        });
        document.querySelectorAll('.delete-flow-btn').forEach(btn => {
            btn.addEventListener('click', async () => {
                const flowID = btn.getAttribute('data-id');
                try {
                    const response = await fetch(`/api/v1/flows/${flowID}`, {
                        method: 'DELETE'
                    });
                    if (!response.ok) {
                        throw new Error(`HTTP error! status: ${response.status}`);
                    }
                    await loadFlows();
                } catch (error) {
                    console.error('Error deleting flow:', error);
                    alert('Failed to delete flow. Please try again.');
                }
            });
        });
    } catch (error) {
        console.error('Error loading flows:', error);
        alert('Failed to load flows. Please try again.');
    }
}

// Load instances when the page loads
showMainDashboard();