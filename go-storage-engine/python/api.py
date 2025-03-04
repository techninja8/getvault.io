from flask import Flask, request, jsonify, send_file
import subprocess
import os

app = Flask(__name__)

STORAGE_ENGINE_PATH = "~/getvault.io/go-storage-engine/cmd"  # Path to the compiled Go storage engine binary

@app.route('/store', methods=['POST'])
def store():
    file = request.files['file']
    file_path = os.path.join("/tmp", file.filename)
    file.save(file_path)
    
    result = subprocess.run([STORAGE_ENGINE_PATH, "store", file_path], stdout=subprocess.PIPE)
    data_id = result.stdout.decode('utf-8').strip()
    
    return jsonify({"data_id": data_id})

@app.route('/retrieve/<data_id>', methods=['GET'])
def retrieve(data_id):
    output_path = os.path.join("/tmp", f"{data_id}.retrieved")
    
    result = subprocess.run([STORAGE_ENGINE_PATH, "retrieve", data_id, output_path], stdout=subprocess.PIPE)
    
    if result.returncode == 0:
        return send_file(output_path, as_attachment=True)
    else:
        return jsonify({"error": "Failed to retrieve file"}), 500

if __name__ == "__main__":
    app.run(port=8080)