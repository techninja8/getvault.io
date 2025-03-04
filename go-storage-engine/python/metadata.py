import psycopg2
from psycopg2 import sql

def create_table():
    conn = psycopg2.connect("dbname=yourdbname user=youruser password=yourpassword host=yourhost")
    cur = conn.cursor()
    cur.execute("""
        CREATE TABLE IF NOT EXISTS metadata (
            data_id VARCHAR PRIMARY KEY,
            file_name VARCHAR,
            stored_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )
    """)
    conn.commit()
    cur.close()
    conn.close()

def insert_metadata(data_id, file_name):
    conn = psycopg2.connect("dbname=yourdbname user=youruser password=yourpassword host=yourhost")
    cur = conn.cursor()
    cur.execute("""
        INSERT INTO metadata (data_id, file_name) VALUES (%s, %s)
    """, (data_id, file_name))
    conn.commit()
    cur.close()
    conn.close()

def get_metadata(data_id):
    conn = psycopg2.connect("dbname=yourdbname user=youruser password=yourpassword host=yourhost")
    cur = conn.cursor()
    cur.execute("""
        SELECT * FROM metadata WHERE data_id = %s
    """, (data_id,))
    metadata = cur.fetchone()
    cur.close()
    conn.close()
    return metadata

if __name__ == "__main__":
    create_table()