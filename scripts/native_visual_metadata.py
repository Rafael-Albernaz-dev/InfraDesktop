"""Keep browser measurements separate from actual native capture facts."""
def native_metadata(row):
    browser_keys=('bounds','reachability','errors')
    return {**{key:value for key,value in row.items() if key not in browser_keys},
            'browser_supplement':{key:row[key] for key in browser_keys if key in row},
            'native_reachability':'NOT RUN - browser geometry/hit tests/errors are supplemental only'}
