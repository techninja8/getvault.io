import click
import requests
import json

API_URL = "http://localhost:8080"  # Assuming you have a REST API for the Go storage engine

@click.group()
def cli():
    pass

@click.command()
@click.argument('file_path', type=click.Path(exists=True))
def store(file_path):
    """Store a file in the storage engine."""
    with open(file_path, 'rb') as file:
        data = file.read()
        response = requests.post(f"{API_URL}/store", files={"file": data})
    
    if response.status_code == 200:
        data_id = response.json().get("data_id")
        click.echo(f"File stored successfully. DataID: {data_id}")
    else:
        click.echo("Failed to store file.")

@click.command()
@click.argument('data_id')
@click.argument('output_path', type=click.Path())
def retrieve(data_id, output_path):
    """Retrieve a file from the storage engine using its DataID."""
    response = requests.get(f"{API_URL}/retrieve/{data_id}")
    
    if response.status_code == 200:
        with open(output_path, 'wb') as file:
            file.write(response.content)
        click.echo(f"File retrieved successfully and saved to {output_path}")
    else:
        click.echo("Failed to retrieve file.")

cli.add_command(store)
cli.add_command(retrieve)

if __name__ == "__main__":
    cli()