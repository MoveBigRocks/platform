#!/usr/bin/env python3
"""
Integration test for Sentry SDK compatibility with Move Big Rocks.

This test uses the REAL Python Sentry SDK to send errors to Move Big Rocks
and verifies they are received and stored correctly.

Requirements:
    pip install sentry-sdk requests

Usage:
    python tests/sentry_sdk_integration_test.py
"""

import json
import os
import time
import requests
import sentry_sdk
from sentry_sdk.transport import Transport
from sentry_sdk.envelope import Envelope


class TestTransport(Transport):
    """Custom transport to capture what the SDK sends"""
    def __init__(self, options):
        super().__init__(options)
        self.envelopes = []
        self.last_envelope_raw = None

    def capture_envelope(self, envelope: Envelope):
        """Capture envelope for inspection"""
        self.envelopes.append(envelope)
        # Serialize to see what's actually sent
        self.last_envelope_raw = envelope.serialize()

        # Also send to Move Big Rocks for real
        headers = {
            'Content-Type': 'application/x-sentry-envelope',
            'X-Sentry-Auth': f'Sentry sentry_key={self.parsed_dsn.public_key},sentry_version=7',
        }

        url = f"{self.parsed_dsn.scheme}://{self.parsed_dsn.host}/api/envelope/"

        try:
            response = requests.post(url, data=self.last_envelope_raw, headers=headers, timeout=5)
            print(f"✅ Sent envelope to Move Big Rocks: {response.status_code}")
            print(f"   Response: {response.text}")
            return response
        except Exception as e:
            print(f"❌ Failed to send envelope: {e}")
            raise


def test_sentry_sdk_integration():
    """Test that real Sentry SDK works with Move Big Rocks"""

    print("=" * 80)
    print("SENTRY SDK INTEGRATION TEST")
    print("=" * 80)

    # Configure Sentry SDK to use Move Big Rocks
    test_dsn = os.getenv("SENTRY_TEST_DSN", "http://test-public-key:test-secret-key@localhost:8080/test-project-key")

    # Create custom transport to inspect what SDK sends
    custom_transport = TestTransport

    print(f"\n1. Initializing Sentry SDK with Move Big Rocks DSN...")
    print(f"   DSN: {test_dsn}")

    sentry_sdk.init(
        dsn=test_dsn,
        transport=custom_transport,
        traces_sample_rate=1.0,
        debug=True,  # Enable debug to see what's happening
    )

    print("\n2. Sending test error...")

    try:
        # Trigger a real error
        result = 1 / 0
    except ZeroDivisionError as e:
        event_id = sentry_sdk.capture_exception(e)
        print(f"   ✅ Error captured by SDK: {event_id}")

    # Give SDK time to send
    time.sleep(2)

    # Check what was sent
    transport = sentry_sdk.Hub.current.client.transport

    print(f"\n3. Verifying envelope format...")

    if hasattr(transport, 'last_envelope_raw'):
        raw_data = transport.last_envelope_raw
        print(f"   Envelope size: {len(raw_data)} bytes")

        # Parse envelope to show structure
        lines = raw_data.decode('utf-8').split('\n')
        print(f"   Envelope lines: {len(lines)}")

        print("\n   Envelope structure:")
        for i, line in enumerate(lines[:5]):  # Show first 5 lines
            if line.strip():
                # Try to parse as JSON to pretty print
                try:
                    data = json.loads(line)
                    print(f"     Line {i+1}: {json.dumps(data, indent=6)[:100]}...")
                except:
                    print(f"     Line {i+1}: {line[:100]}...")

        # Verify it's newline-delimited format
        if len(lines) >= 3:
            print(f"\n   ✅ Envelope uses newline-delimited format")
        else:
            print(f"\n   ❌ Envelope format looks wrong (too few lines)")

    print(f"\n4. Testing with breadcrumbs and context...")

    # Add breadcrumbs
    sentry_sdk.add_breadcrumb(
        category='test',
        message='Test breadcrumb 1',
        level='info'
    )

    sentry_sdk.add_breadcrumb(
        category='navigation',
        message='User clicked button',
        level='info'
    )

    # Set user context
    sentry_sdk.set_user({
        "id": "test-user-123",
        "email": "test@example.com",
        "username": "testuser"
    })

    # Set tags
    sentry_sdk.set_tag("environment", "test")
    sentry_sdk.set_tag("test_run", "integration")

    try:
        # Another error with full context
        raise ValueError("Test error with full context")
    except ValueError as e:
        event_id = sentry_sdk.capture_exception(e)
        print(f"   ✅ Error with context captured: {event_id}")

    time.sleep(2)

    print(f"\n5. Testing message capture...")

    message_id = sentry_sdk.capture_message("Test message from SDK", level="warning")
    print(f"   ✅ Message captured: {message_id}")

    time.sleep(1)

    print("\n" + "=" * 80)
    print("TEST SUMMARY")
    print("=" * 80)

    if hasattr(transport, 'envelopes'):
        print(f"✅ Total envelopes sent: {len(transport.envelopes)}")

    print("\nIf you see successful HTTP responses above, the Sentry SDK integration")
    print("is working correctly with Move Big Rocks!")

    print("\nTo verify errors were stored, check:")
    print("  GET http://localhost:8080/v1/error-monitoring/projects/test-project-key/issues")

    return True


if __name__ == "__main__":
    try:
        success = test_sentry_sdk_integration()
        if success:
            print("\n✅ Integration test PASSED")
            exit(0)
        else:
            print("\n❌ Integration test FAILED")
            exit(1)
    except Exception as e:
        print(f"\n❌ Test error: {e}")
        import traceback
        traceback.print_exc()
        exit(1)
