import boto3
import csv

# Configuration
TABLE_NAME = 'Devices'
CSV_FILE_PATH = 'devices.csv'
LOCAL_ENDPOINT = 'http://localhost:8000'
PARTITION_KEY = 'AssetTag'  # Replace with your actual ID column name

def import_all_as_strings():
    # Connect to local DynamoDB
    dynamodb = boto3.resource(
        'dynamodb',
        endpoint_url=LOCAL_ENDPOINT,
        region_name='us-east-1',
        aws_access_key_id='local',
        aws_secret_access_key='local'
    )
    table = dynamodb.Table(TABLE_NAME)

    print(f"Starting import into {TABLE_NAME}...")

    with open(CSV_FILE_PATH, 'r', encoding='utf-8-sig') as csvfile:
        reader = csv.DictReader(csvfile)
        
        with table.batch_writer() as batch:
            for line_number, row in enumerate(reader, start=1):
                # 1. Force every key and value to be a string and strip whitespace
                # Filter out empty keys (fixes 'Empty attribute name' error)
                item = {
                    str(k).strip(): str(v).strip() 
                    for k, v in row.items() 
                    if k and k.strip()
                }

                # 2. VALIDATION: Ensure the Partition Key exists
                if not item.get(PARTITION_KEY):
                    print(f"Line {line_number}: Skipped (Missing {PARTITION_KEY})")
                    continue

                # 3. Write to DynamoDB
                try:
                    batch.put_item(Item=item)
                except Exception as e:
                    print(f"Line {line_number}: Failed to insert. Error: {e}")
                    
    print("Done! All data has been sent as strings.")

if __name__ == "__main__":
    import_all_as_strings()