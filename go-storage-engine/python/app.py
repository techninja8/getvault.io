from flask import Flask, request, jsonify
from metadata_manager import MetadataManager
from auth import Authentication, AccessControl
import os

app = Flask(__name__)

# Initialize MetadataManager with PostgreSQL and Redis configurations
pg_config = {
    'dbname': os.getenv('PG_DBNAME', 'metadata_db'),
    'user': os.getenv('PG_USER', 'user'),
    'password': os.getenv('PG_PASSWORD', 'password'),
    'host': os.getenv('PG_HOST', 'localhost'),
    'port': os.getenv('PG_PORT', 5432)
}

redis_config = {
    'host': os.getenv('REDIS_HOST', 'localhost'),
    'port': os.getenv('REDIS_PORT', 6379)
}

metadata_manager = MetadataManager(pg_config, redis_config)

@app.route('/metadata', methods=['POST'])
def post_metadata():
    data = request.json
    data_id = data.get('dataID')
    metadata = data.get('metadata')
    metadata_manager.track_file(data_id, metadata)
    return jsonify({'message': 'Metadata stored successfully'}), 201

@app.route('/metadata', methods=['GET'])
def get_metadata():
    data_id = request.args.get('dataID')
    metadata = metadata_manager.retrieve_metadata(data_id)
    if metadata:
        return jsonify({'metadata': metadata}), 200
    return jsonify({'error': 'Metadata not found'}), 404

@app.route('/login', methods=['POST'])
def login():
    credentials = request.json
    token = Authentication.login(credentials)
    if token:
        return jsonify({'token': token}), 200
    return jsonify({'error': 'Invalid credentials'}), 401

@app.route('/logout', methods=['POST'])
def logout():
    token = request.json.get('token')
    if Authentication.logout(token):
        return jsonify({'message': 'Logged out successfully'}), 200
    return jsonify({'error': 'Invalid token'}), 400

if __name__ == '__main__':
    app.run(debug=True)