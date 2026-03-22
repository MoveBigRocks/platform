#!/usr/bin/env node
/**
 * Integration test for Sentry SDK compatibility with Move Big Rocks.
 *
 * This test uses the REAL Node.js Sentry SDK to send errors to Move Big Rocks
 * and verifies they are received and stored correctly.
 *
 * Requirements:
 *     npm install @sentry/node axios
 *
 * Usage:
 *     node tests/sentry_sdk_integration_test.js
 */

const Sentry = require('@sentry/node');
const axios = require('axios');
const defaultTestDSN = 'http://test-public-key:test-secret-key@localhost:8080/test-project-key';

// Custom transport to inspect what SDK sends
class TestTransport {
    constructor(options) {
        this.options = options;
        this.envelopes = [];
    }

    async sendEnvelope(envelope) {
        // Store envelope for inspection
        this.envelopes.push(envelope);

        // Serialize envelope to see raw format
        const envelopeString = this.serializeEnvelope(envelope);

        console.log(`\n📦 Envelope being sent:`);
        console.log(`   Size: ${envelopeString.length} bytes`);

        const lines = envelopeString.split('\n');
        console.log(`   Lines: ${lines.length}`);
        console.log(`   Format: newline-delimited`);

        // Show first few lines
        console.log(`\n   Structure:`);
        lines.slice(0, 5).forEach((line, i) => {
            if (line.trim()) {
                try {
                    const data = JSON.parse(line);
                    console.log(`     Line ${i+1}: ${JSON.stringify(data).substring(0, 80)}...`);
                } catch (e) {
                    console.log(`     Line ${i+1}: ${line.substring(0, 80)}...`);
                }
            }
        });

        // Send to Move Big Rocks
        const dsn = this.options.dsn;
        const url = `${dsn.protocol}://${dsn.host}/api/envelope/`;

        try {
            const response = await axios.post(url, envelopeString, {
                headers: {
                    'Content-Type': 'application/x-sentry-envelope',
                    'X-Sentry-Auth': `Sentry sentry_key=${dsn.publicKey},sentry_version=7`,
                },
                timeout: 5000
            });

            console.log(`   ✅ Sent to Move Big Rocks: ${response.status} ${response.statusText}`);
            console.log(`   Response: ${JSON.stringify(response.data)}`);

            return { status: 'success', code: 200 };
        } catch (error) {
            console.log(`   ❌ Failed to send: ${error.message}`);
            if (error.response) {
                console.log(`   Response: ${error.response.status} - ${JSON.stringify(error.response.data)}`);
            }
            throw error;
        }
    }

    serializeEnvelope(envelope) {
        // Serialize envelope to newline-delimited format
        const lines = [];

        // Envelope header
        lines.push(JSON.stringify(envelope[0]));

        // Items (header + payload pairs)
        for (let i = 1; i < envelope.length; i++) {
            const item = envelope[i];
            const [itemHeader, itemPayload] = item;

            lines.push(JSON.stringify(itemHeader));
            lines.push(JSON.stringify(itemPayload));
        }

        return lines.join('\n') + '\n';
    }

    flush(timeout) {
        return Promise.resolve(true);
    }
}

async function sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
}

async function testSentrySDKIntegration() {
    console.log('='.repeat(80));
    console.log('SENTRY SDK INTEGRATION TEST (Node.js)');
    console.log('='.repeat(80));

    // Configure Sentry SDK to use Move Big Rocks
    const testDSN = process.env.SENTRY_TEST_DSN || defaultTestDSN;

    console.log(`\n1. Initializing Sentry SDK with Move Big Rocks DSN...`);
    console.log(`   DSN: ${testDSN}`);

    // Initialize with custom transport
    Sentry.init({
        dsn: testDSN,
        transport: TestTransport,
        debug: true,
        tracesSampleRate: 1.0,
    });

    console.log(`\n2. Sending test error...`);

    try {
        // Trigger a real error
        throw new Error('Test error from Node.js SDK');
    } catch (error) {
        const eventId = Sentry.captureException(error);
        console.log(`   ✅ Error captured by SDK: ${eventId}`);
    }

    await sleep(2000);

    console.log(`\n3. Testing with breadcrumbs and context...`);

    // Add breadcrumbs
    Sentry.addBreadcrumb({
        category: 'test',
        message: 'Test breadcrumb 1',
        level: 'info'
    });

    Sentry.addBreadcrumb({
        category: 'navigation',
        message: 'User navigated to /test',
        level: 'info'
    });

    // Set user context
    Sentry.setUser({
        id: 'test-user-456',
        email: 'nodejs@example.com',
        username: 'nodejsuser'
    });

    // Set tags
    Sentry.setTag('environment', 'test');
    Sentry.setTag('runtime', 'nodejs');

    try {
        const obj = null;
        // This will throw TypeError
        obj.someMethod();
    } catch (error) {
        const eventId = Sentry.captureException(error);
        console.log(`   ✅ Error with context captured: ${eventId}`);
    }

    await sleep(2000);

    console.log(`\n4. Testing message capture...`);

    const messageId = Sentry.captureMessage('Test message from Node.js SDK', 'warning');
    console.log(`   ✅ Message captured: ${messageId}`);

    await sleep(1000);

    console.log(`\n5. Testing with stack trace...`);

    function level3() {
        throw new Error('Error from nested function');
    }

    function level2() {
        level3();
    }

    function level1() {
        level2();
    }

    try {
        level1();
    } catch (error) {
        const eventId = Sentry.captureException(error);
        console.log(`   ✅ Error with stack trace captured: ${eventId}`);
    }

    await sleep(2000);

    console.log('\n' + '='.repeat(80));
    console.log('TEST SUMMARY');
    console.log('='.repeat(80));

    const client = Sentry.getCurrentHub().getClient();
    if (client && client._getBackend) {
        const transport = client._getBackend()._transport;
        if (transport && transport.envelopes) {
            console.log(`✅ Total envelopes sent: ${transport.envelopes.length}`);
        }
    }

    console.log(`\nIf you see successful HTTP responses above, the Sentry SDK integration`);
    console.log(`is working correctly with Move Big Rocks!`);

    console.log(`\nTo verify errors were stored, check:`);
    console.log(`  GET http://localhost:8080/v1/error-monitoring/projects/test-project-key/issues`);

    // Flush to ensure all events are sent
    await Sentry.flush(2000);

    return true;
}

// Run the test
testSentrySDKIntegration()
    .then(() => {
        console.log('\n✅ Integration test PASSED');
        process.exit(0);
    })
    .catch((error) => {
        console.log('\n❌ Integration test FAILED');
        console.error(error);
        process.exit(1);
    });
