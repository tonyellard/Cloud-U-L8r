#!/usr/bin/env python3
"""
Test new features: multipart uploads, range requests, batch delete, pagination
"""

import boto3
import io
import os
from botocore.exceptions import ClientError

ENDPOINT_URL = 'http://localhost:9300'
BUCKET_NAME = 'test-bucket'

s3 = boto3.client(
    's3',
    endpoint_url=ENDPOINT_URL,
    aws_access_key_id='test',
    aws_secret_access_key='test',
    region_name='us-east-1'
)

def test_multipart_upload():
    """Test multipart upload functionality"""
    print("Testing Multipart Upload...", end=' ')
    try:
        key = 'multipart-test.txt'
        
        # Create multipart upload
        response = s3.create_multipart_upload(
            Bucket=BUCKET_NAME,
            Key=key,
            ContentType='text/plain',
            Metadata={'test': 'multipart'}
        )
        upload_id = response['UploadId']
        print(f"\n  Created upload: {upload_id}")
        
        # Upload parts
        parts = []
        part_data = [
            b'This is part 1. ' * 100,
            b'This is part 2. ' * 100,
            b'This is part 3. ' * 100,
        ]
        
        for i, data in enumerate(part_data, start=1):
            response = s3.upload_part(
                Bucket=BUCKET_NAME,
                Key=key,
                UploadId=upload_id,
                PartNumber=i,
                Body=data
            )
            parts.append({
                'PartNumber': i,
                'ETag': response['ETag']
            })
            print(f"  Uploaded part {i}: {response['ETag']}")
        
        # Complete multipart upload
        response = s3.complete_multipart_upload(
            Bucket=BUCKET_NAME,
            Key=key,
            UploadId=upload_id,
            MultipartUpload={'Parts': parts}
        )
        print(f"  Completed: {response['ETag']}")
        
        # Verify the object exists and has correct size
        response = s3.head_object(Bucket=BUCKET_NAME, Key=key)
        expected_size = sum(len(d) for d in part_data)
        assert response['ContentLength'] == expected_size, f"Size mismatch: {response['ContentLength']} != {expected_size}"
        
        print("✓ PASSED")
        return True
    except Exception as e:
        print(f"✗ FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False

def test_range_request():
    """Test range requests"""
    print("Testing Range Requests...", end=' ')
    try:
        key = 'range-test.txt'
        content = b'0123456789' * 10  # 100 bytes
        
        # Upload file
        s3.put_object(Bucket=BUCKET_NAME, Key=key, Body=content)
        
        # Request range
        response = s3.get_object(Bucket=BUCKET_NAME, Key=key, Range='bytes=10-19')
        partial_content = response['Body'].read()
        
        # Verify we got the right bytes
        expected = content[10:20]
        assert partial_content == expected, f"Range content mismatch: {partial_content} != {expected}"
        assert response['ContentLength'] == 10, "Content length mismatch"
        assert 'ContentRange' in response, "ContentRange header missing"
        
        print("✓ PASSED")
        return True
    except Exception as e:
        print(f"✗ FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False

def test_batch_delete():
    """Test batch delete functionality"""
    print("Testing Batch Delete...", end=' ')
    try:
        # Upload multiple objects
        keys = [f'batch-delete-{i}.txt' for i in range(5)]
        for key in keys:
            s3.put_object(Bucket=BUCKET_NAME, Key=key, Body=b'test')
        
        # Delete them all at once
        response = s3.delete_objects(
            Bucket=BUCKET_NAME,
            Delete={
                'Objects': [{'Key': key} for key in keys]
            }
        )
        
        # Verify all were deleted
        deleted = {obj['Key'] for obj in response.get('Deleted', [])}
        assert len(deleted) == 5, f"Expected 5 deletions, got {len(deleted)}"
        
        # Verify objects are gone
        for key in keys:
            try:
                s3.head_object(Bucket=BUCKET_NAME, Key=key)
                raise Exception(f"Object {key} still exists after delete")
            except ClientError as e:
                if e.response['Error']['Code'] != '404':
                    raise
        
        print("✓ PASSED")
        return True
    except Exception as e:
        print(f"✗ FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False

def test_list_objects_v1():
    """Test ListObjects V1 API"""
    print("Testing ListObjects V1...", end=' ')
    try:
        # Upload objects
        for i in range(5):
            s3.put_object(Bucket=BUCKET_NAME, Key=f'v1-test-{i:02d}.txt', Body=b'test')
        
        # List with V1 API (boto3 uses V2 by default, but we can test through pagination)
        response = s3.list_objects(Bucket=BUCKET_NAME, Prefix='v1-test-', MaxKeys=3)
        
        assert 'Contents' in response, "Contents missing"
        assert len(response['Contents']) == 3, f"Expected 3 objects, got {len(response['Contents'])}"
        assert response['IsTruncated'] == True, "Should be truncated"
        assert 'NextMarker' in response, "NextMarker missing"
        
        # Get next page
        response = s3.list_objects(
            Bucket=BUCKET_NAME,
            Prefix='v1-test-',
            Marker=response['NextMarker'],
            MaxKeys=3
        )
        assert len(response['Contents']) == 2, f"Expected 2 objects in page 2, got {len(response['Contents'])}"
        
        print("✓ PASSED")
        return True
    except Exception as e:
        print(f"✗ FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False

def test_list_objects_v2_pagination():
    """Test ListObjectsV2 with pagination"""
    print("Testing ListObjectsV2 Pagination...", end=' ')
    try:
        # Upload many objects
        for i in range(10):
            s3.put_object(Bucket=BUCKET_NAME, Key=f'v2-test-{i:02d}.txt', Body=b'test')
        
        # List with pagination
        response = s3.list_objects_v2(Bucket=BUCKET_NAME, Prefix='v2-test-', MaxKeys=4)
        
        assert 'Contents' in response, "Contents missing"
        assert len(response['Contents']) == 4, f"Expected 4 objects, got {len(response['Contents'])}"
        assert response['IsTruncated'] == True, "Should be truncated"
        assert 'NextContinuationToken' in response, "NextContinuationToken missing"
        
        # Get next page
        all_keys = [obj['Key'] for obj in response['Contents']]
        
        while response.get('IsTruncated'):
            response = s3.list_objects_v2(
                Bucket=BUCKET_NAME,
                Prefix='v2-test-',
                ContinuationToken=response['NextContinuationToken'],
                MaxKeys=4
            )
            all_keys.extend([obj['Key'] for obj in response.get('Contents', [])])
        
        assert len(all_keys) == 10, f"Expected 10 total objects, got {len(all_keys)}"
        
        print("✓ PASSED")
        return True
    except Exception as e:
        print(f"✗ FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False

def test_abort_multipart():
    """Test aborting a multipart upload"""
    print("Testing Abort Multipart Upload...", end=' ')
    try:
        key = 'abort-test.txt'
        
        # Create multipart upload
        response = s3.create_multipart_upload(Bucket=BUCKET_NAME, Key=key)
        upload_id = response['UploadId']
        
        # Upload a part
        s3.upload_part(
            Bucket=BUCKET_NAME,
            Key=key,
            UploadId=upload_id,
            PartNumber=1,
            Body=b'test data'
        )
        
        # Abort the upload
        s3.abort_multipart_upload(
            Bucket=BUCKET_NAME,
            Key=key,
            UploadId=upload_id
        )
        
        # Verify the object doesn't exist
        try:
            s3.head_object(Bucket=BUCKET_NAME, Key=key)
            raise Exception("Object should not exist after abort")
        except ClientError as e:
            if e.response['Error']['Code'] != '404':
                raise
        
        print("✓ PASSED")
        return True
    except Exception as e:
        print(f"✗ FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False

def main():
    print("=" * 70)
    print("ess-three Advanced Features Tests")
    print("=" * 70)
    print(f"Endpoint: {ENDPOINT_URL}")
    print(f"Bucket: {BUCKET_NAME}")
    print("=" * 70)
    
    results = []
    results.append(test_multipart_upload())
    results.append(test_abort_multipart())
    results.append(test_range_request())
    results.append(test_batch_delete())
    results.append(test_list_objects_v1())
    results.append(test_list_objects_v2_pagination())
    
    print("=" * 70)
    passed = sum(results)
    total = len(results)
    print(f"Results: {passed}/{total} tests passed")
    print("=" * 70)
    
    if passed == total:
        print("✓ All advanced features working!")
        return 0
    else:
        print("✗ Some tests failed")
        return 1

if __name__ == '__main__':
    exit(main())
