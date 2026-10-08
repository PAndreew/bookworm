#!/usr/bin/env python3
"""Run as root on the existing VPS. Never prints or writes plaintext credentials to Git.
Reuses the old Potpuri Stripe account/€24 annual price with a capped €4 discount.
No checkout, subscription or charge is created by this setup.
"""
import base64
import json
import os
from pathlib import Path
import subprocess
from urllib.error import HTTPError
from urllib.parse import urlencode
from urllib.request import Request, urlopen

APP = 'potpuri-reader'
COUPON = 'potpuri_reader_founding_50'
ROOT = Path('/opt/bookworm')
VAULT = Path('/opt/potpuri-reader-vault')

def stripe(key, method, path, params=None, idem=None):
    headers = {'Authorization': 'Basic ' + base64.b64encode((key + ':').encode()).decode(), 'Stripe-Version': '2024-06-20'}
    if idem:
        headers['Idempotency-Key'] = idem
    request = Request('https://api.stripe.com/v1/' + path, data=urlencode(params).encode() if params else None, headers=headers, method=method)
    with urlopen(request, timeout=20) as response:
        return json.load(response)

def encrypt(name, secret):
    VAULT.mkdir(mode=0o700, exist_ok=True)
    dest = VAULT / (name + '.cred')
    if dest.exists():
        return
    subprocess.run(['systemd-creds', 'encrypt', '--with-key=host', '--name=' + name, '-', str(dest)], input=secret.encode(), check=True, stdout=subprocess.DEVNULL)
    os.chmod(dest, 0o600)

if __name__ == '__main__':
    assert os.geteuid() == 0, 'Run as root on VPS'
    container = json.loads(subprocess.check_output(['docker', 'inspect', 'potpuri-app']))[0]
    env = dict(item.split('=', 1) for item in container['Config']['Env'] if '=' in item)
    key, price_id = env['STRIPE_SECRET_KEY'], env['STRIPE_PRICE_ID']
    price = stripe(key, 'GET', 'prices/' + price_id)
    assert price['currency'] == 'eur' and price['unit_amount'] == 2400 and price['active'] and price['livemode']
    assert price['recurring']['interval'] == 'year' and price['recurring']['interval_count'] == 1
    try:
        coupon = stripe(key, 'GET', 'coupons/' + COUPON)
    except HTTPError as error:
        if error.code != 404:
            raise RuntimeError('Could not verify Stripe founding coupon') from None
        coupon = stripe(key, 'POST', 'coupons', {
            'id': COUPON, 'name': 'Founding Plus — €4 annual discount', 'amount_off': 400,
            'currency': 'eur', 'duration': 'forever', 'max_redemptions': 50,
            'applies_to[products][0]': price['product'], 'metadata[app]': APP,
        }, 'reader-founding-coupon-v1')
    assert coupon['amount_off'] == 400 and coupon['currency'] == 'eur' and coupon['max_redemptions'] == 50 and coupon['duration'] == 'forever'
    assert coupon.get('applies_to', {}).get('products') == [price['product']]
    stripe(key, 'POST', 'products/' + price['product'], {
        'name': 'Potpuri Plus',
        'description': 'Founding membership: 100 AI explanations or translations per calendar month. €20/year after the founding discount.',
        'metadata[app]': APP,
    }, 'reader-founding-product-v1')
    # Persist non-secret price/coupon IDs only. Preserve all unrelated environment settings.
    path = ROOT / '.env'
    lines = path.read_text().splitlines()
    updates = {'BW_STRIPE_PRICE': price_id, 'BW_STRIPE_COUPON': COUPON}
    lines = [line for line in lines if line.split('=', 1)[0] not in updates]
    path.write_text('\n'.join(lines + [key + '=' + value for key, value in updates.items()]) + '\n')
    os.chmod(path, 0o600)
    encrypt('stripe_key', key)
    # Each webhook endpoint has its own signing secret, even on the reused account.
    secret_path = VAULT / 'stripe_webhook.cred'
    if not secret_path.exists():
        endpoints = stripe(key, 'GET', 'webhook_endpoints?limit=100')['data']
        target = 'https://potpuri.cc/v1/billing/webhook'
        assert not any(e['url'] == target for e in endpoints), 'Endpoint exists without its saved signing secret; recover it instead of duplicating it'
        endpoint = stripe(key, 'POST', 'webhook_endpoints', {
            'url': target, 'api_version': '2024-06-20', 'description': 'Potpuri reader founding membership',
            'enabled_events[0]': 'checkout.session.completed',
            'enabled_events[1]': 'customer.subscription.created',
            'enabled_events[2]': 'customer.subscription.updated',
            'enabled_events[3]': 'customer.subscription.deleted',
        }, 'reader-founding-webhook-v1')
        encrypt('stripe_webhook', endpoint['secret'])
    print('Stripe configured: €20/year net founding offer, 50-redemption cap. No charges created.')
