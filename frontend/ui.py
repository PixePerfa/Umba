from flask import Flask, render_template, request, redirect, url_for, send_from_directory
import requests

app = Flask(__name__, static_folder='static', template_folder='templates')

BACKEND_URL = "http://localhost:8080"

@app.route('/')
def index():
    return render_template('index.html')

@app.route('/static/<path:path>')
def send_static(path):
    return send_from_directory('static', path)

@app.route('/api/v1/instances', methods=['GET'])
def get_instances():
    response = requests.get(f"{BACKEND_URL}/api/v1/instances")
    return response.json()

@app.route('/api/v1/instances', methods=['POST'])
def create_instance():
    data = request.json
    response = requests.post(f"{BACKEND_URL}/api/v1/instances", json=data)
    return response.json()

@app.route('/api/v1/instances/<id>', methods=['DELETE'])
def delete_instance(id):
    response = requests.delete(f"{BACKEND_URL}/api/v1/instances/{id}")
    return response.json()

@app.route('/api/v1/instances/<id>/status', methods=['PUT'])
def update_instance_status(id):
    data = request.json
    response = requests.put(f"{BACKEND_URL}/api/v1/instances/{id}/status", json=data)
    return response.json()

@app.route('/api/v1/instances/<id>/screenshot', methods=['GET'])
def get_instance_screenshot(id):
    response = requests.get(f"{BACKEND_URL}/api/v1/instances/{id}/screenshot")
    return response.content, 200, {'Content-Type': 'image/png'}

@app.route('/api/v1/flows', methods=['GET'])
def get_flows():
    response = requests.get(f"{BACKEND_URL}/api/v1/flows")
    return response.json()

@app.route('/api/v1/flows', methods=['POST'])
def create_flow():
    data = request.json
    response = requests.post(f"{BACKEND_URL}/api/v1/flows", json=data)
    return response.json()

@app.route('/api/v1/flows/<id>', methods=['DELETE'])
def delete_flow(id):
    response = requests.delete(f"{BACKEND_URL}/api/v1/flows/{id}")
    return response.json()

@app.route('/api/v1/flows/execute', methods=['POST'])
def execute_flows():
    data = request.json
    response = requests.post(f"{BACKEND_URL}/api/v1/flows/execute", json=data)
    return response.json()

if __name__ == '__main__':
    app.run(port=5000)