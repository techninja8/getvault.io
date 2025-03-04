import redis
import psycopg2
from psycopg2.extras import RealDictCursor

class MetadataManager:
    def __init__(self, pg_config, redis_config):
        self.pg_conn = psycopg2.connect(**pg_config)
        self.redis_conn = redis.Redis(**redis_config)

    def track_file(self, data_id, metadata):
        # Store in PostgreSQL
        with self.pg_conn.cursor() as cursor:
            cursor.execute(
                "INSERT INTO metadata (data_id, metadata) VALUES (%s, %s) ON CONFLICT (data_id) DO UPDATE SET metadata = %s",
                (data_id, metadata, metadata)
            )
        self.pg_conn.commit()
        # Cache in Redis
        self.redis_conn.set(data_id, metadata)

    def retrieve_metadata(self, data_id):
        # First check Redis cache
        metadata = self.redis_conn.get(data_id)
        if metadata:
            return metadata
        # Fallback to PostgreSQL
        with self.pg_conn.cursor(cursor_factory=RealDictCursor) as cursor:
            cursor.execute("SELECT metadata FROM metadata WHERE data_id = %s", (data_id,))
            result = cursor.fetchone()
            if result:
                metadata = result['metadata']
                # Cache in Redis for future access
                self.redis_conn.set(data_id, metadata)
                return metadata
        return None