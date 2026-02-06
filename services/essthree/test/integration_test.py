#!/usr/bin/env python3
"""
Integration tests for ess-three S3 emulator.
Tests the emulator using the boto3 AWS SDK.

Requirements:
    pip install boto3

Usage:
    # Start the emulator first
    docker-compose up -d
    
    # Run tests
    python test/integration_test.py
"""

import boto3
import sys
from botocore.exceptions import ClientError

# Configuration
ENDPOINT_URL = 'http://localhost:9300'
BUCKET_NAME = 'test-bucket'
TEST_KEY = 'test-file.txt'
TEST_CONTENT = b'Hello from ess-three!'

def create_s3_client():
    """Create S3 client pointing to local emulator."""
    return boto3.client(
        's3',
        endpoint_url=ENDPOINT_URL,
        aws_access_key_id='test',
        aws_secret_access_key='test',
        region_name='us-east-1'
    )

def test_put_object(s3):
    """Test uploading an object."""
    print("Testing PutObject...", end=' ')
    try:
        response = s3.put_object(
            Bucket=BUCKET_NAME,
            Key=TEST_KEY,
            Body=TEST_CONTENT,
            ContentType='text/plain',
            Metadata={'author': 'test', 'version': '1.0'}
        )
        assert 'ETag' in response
        print("✓ PASSED")
        return True
    except Exception as e:
        print(f"✗ FAILED: {e}")
        return False

def test_get_object(s3):
    """Test downloading an object."""
    print("Testing GetObject...", end=' ')
    try:
        response = s3.get_object(Bucket=BUCKET_NAME, Key=TEST_KEY)
        content = response['Body'].read()
        assert content == TEST_CONTENT
        assert response['ContentType'] == 'text/plain'
        assert 'ETag' in response
        assert 'LastModified' in response
        print("✓ PASSED")
        return True
    except Exception as e:
        print(f"✗ FAILED: {e}")
        return False

def test_head_object(s3):
    """Test getting object metadata."""
    print("Testing HeadObject...", end=' ')
    try:
        response = s3.head_object(Bucket=BUCKET_NAME, Key=TEST_KEY)
        assert response['ContentLength'] == len(TEST_CONTENT), f"Expected {len(TEST_CONTENT)}, got {response['ContentLength']}"
        assert response['ContentType'] == 'text/plain', f"Expected text/plain, got {response['ContentType']}"
        assert 'ETag' in response, "ETag missing"
        assert 'LastModified' in response, "LastModified missing"
        # Metadata keys may be capitalized by HTTP headers
        metadata_lower = {k.lower(): v for k, v in response['Metadata'].items()}
        assert metadata_lower.get('author') == 'test', f"Expected author=test, got {metadata_lower.get('author')}"
        assert metadata_lower.get('version') == '1.0', f"Expected version=1.0, got {metadata_lower.get('version')}"
        print("✓ PASSED")
        return True
    except Exception as e:
        print(f"✗ FAILED: {e}")
        return False

def test_list_objects(s3):
    """Test listing objects."""
    print("Testing ListObjectsV2...", end=' ')
    try:
        # Upload additional test objects with specific prefix
        for i in range(3):
            s3.put_object(
                Bucket=BUCKET_NAME,
                Key=f'listtest-{i}.txt',
                Body=f'Test content {i}'.encode()
            )
        
        response = s3.list_objects_v2(Bucket=BUCKET_NAME)
        assert 'Contents' in response, "Contents key missing from response"
        assert len(response['Contents']) >= 4, f"Expected at least 4 objects, got {len(response['Contents'])}"
        
        # Test with prefix
        response = s3.list_objects_v2(Bucket=BUCKET_NAME, Prefix='listtest-')
        contents_count = len(response.get('Contents', []))
        assert contents_count == 3, f"Expected exactly 3 objects with prefix 'listtest-', got {contents_count}"
        
        print("✓ PASSED")
        return True
    except Exception as e:
        print(f"✗ FAILED: {e}")
        return False

def test_delete_object(s3):
    """Test deleting an object."""
    print("Testing DeleteObject...", end=' ')
    try:
        # Delete the object
        s3.delete_object(Bucket=BUCKET_NAME, Key=TEST_KEY)
        
        # Verify it's gone
        try:
            s3.head_object(Bucket=BUCKET_NAME, Key=TEST_KEY)
            print("✗ FAILED: Object still exists after deletion")
            return False
        except ClientError as e:
            if e.response['Error']['Code'] == '404':
                print("✓ PASSED")
                return True
            raise
    except Exception as e:
        print(f"✗ FAILED: {e}")
        return False

def test_metadata_preservation(s3):
    """Test that custom metadata is preserved."""
    print("Testing metadata preservation...", end=' ')
    try:
        key = 'metadata-test.txt'
        metadata = {
            'author': 'John Doe',
            'version': '2.0',
            'description': 'Test file with metadata'
        }
        
        # Upload with metadata
        s3.put_object(
            Bucket=BUCKET_NAME,
            Key=key,
            Body=b'Test content',
            Metadata=metadata
        )
        
        # Retrieve and verify metadata (case-insensitive comparison)
        response = s3.head_object(Bucket=BUCKET_NAME, Key=key)
        metadata_lower = {k.lower(): v for k, v in response['Metadata'].items()}
        for k, v in metadata.items():
            assert metadata_lower.get(k.lower()) == v, f"Metadata {k} mismatch: expected {v}, got {metadata_lower.get(k.lower())}"
        
        print("✓ PASSED")
        return True
    except Exception as e:
        print(f"✗ FAILED: {e}")
        return False

def main():
    print("=" * 60)
    print("ess-three Integration Tests")
    print("=" * 60)
    print(f"Endpoint: {ENDPOINT_URL}")
    print(f"Bucket: {BUCKET_NAME}")
    print("=" * 60)
    
    s3 = create_s3_client()
    
    # Run tests
    results = []
    results.append(test_put_object(s3))
    results.append(test_get_object(s3))
    results.append(test_head_object(s3))
    results.append(test_list_objects(s3))
    results.append(test_metadata_preservation(s3))
    results.append(test_delete_object(s3))
    
    # Summary
    print("=" * 60)
    passed = sum(results)
    total = len(results)
    print(f"Results: {passed}/{total} tests passed")
    print("=" * 60)
    
    if passed == total:
        print("✓ All tests passed!")
        sys.exit(0)
    else:
        print("✗ Some tests failed")
        sys.exit(1)

if __name__ == '__main__':
    main()
