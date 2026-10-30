// Same-origin authenticated API only. No certificate, password or tax decision is
// accepted here. A saved profile is a draft, never evidence of homologation.
export class FiscalCustomerError extends Error {
  constructor(code, status = 0) { super(code); this.code = code; this.status = status; }
}
export const states = Object.freeze('AC AL AP AM BA CE DF ES GO MA MT MS MG PA PB PR PE PI RJ RN RS RO RR SC SP SE TO'.split(' '));
const id = value => typeof value === 'string' && /^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$/.test(value);
const fields = ['operation_id','profile_id','expected_revision','cnpj','legal_name','state','municipality_ibge','state_registration','crt','model','environment','series'];
export function profileMatches(profile, input) {
  return profile?.revision === input.expected_revision + 1 && fields.every(key => profile.input?.[key] === input[key]);
}
export function createFiscalCustomer({token, isCurrent, fetcher = fetch}) {
  const current = () => { if (!token || !isCurrent()) throw new FiscalCustomerError('session', 401); };
  async function request(path, input) {
    current();
    const controller = new AbortController(), timer = setTimeout(() => controller.abort(), 15000);
    try {
      const response = await fetcher('/local/v1/fiscal/' + path, {
        method: input ? 'POST' : 'GET', credentials: 'omit', cache: 'no-store', signal: controller.signal,
        headers: {Authorization: 'Bearer ' + token, ...(input ? {'Content-Type':'application/json'} : {})},
        ...(input ? {body: JSON.stringify(input)} : {}),
      });
      current();
      if (!response.ok) throw new FiscalCustomerError(response.status === 401 ? 'session' : response.status === 403 ? 'denied' : response.status === 404 ? 'missing' : response.status === 409 ? 'conflict' : 'request', response.status);
      const text = await response.text(); current();
      if (text.length > 262144) throw new FiscalCustomerError('response');
      let value; try { value = JSON.parse(text); } catch { throw new FiscalCustomerError('response'); }
      if (!value || typeof value !== 'object' || Array.isArray(value)) throw new FiscalCustomerError('response');
      return value;
    } catch (error) {
      if (error instanceof FiscalCustomerError) throw error;
      throw new FiscalCustomerError('network');
    } finally { clearTimeout(timer); }
  }
  function validateProfile(value) {
    if (!value.input || !id(value.input.profile_id) || !Number.isSafeInteger(value.revision) || value.revision < 1 ||
        !fields.every(key => ['expected_revision','series'].includes(key) ? Number.isSafeInteger(value.input[key]) : typeof value.input[key] === 'string') ||
        !Array.isArray(value.readiness?.blockers) || !value.readiness.blockers.every(v => typeof v === 'string')) throw new FiscalCustomerError('response');
    return value;
  }
  return {
    async onboarding(profileID) {
      if (!id(profileID)) throw new FiscalCustomerError('invalid');
      const value = await request('onboarding/' + encodeURIComponent(profileID));
      const flags = ['configuration_valid','production_blocked','transmission_performed','official_authorization'];
      const codes = ['onboarding_configuration_required','onboarding_official_verification_required','onboarding_production_blocked'];
      if (Object.keys(value).sort().join() !== [...flags,'profile_revision','blockers'].sort().join() ||
          !flags.every(key => typeof value[key] === 'boolean') || value.production_blocked !== true ||
          value.transmission_performed !== false || value.official_authorization !== false ||
          !Number.isSafeInteger(value.profile_revision) || value.profile_revision < 1 ||
          !Array.isArray(value.blockers) || !value.blockers.every(code => codes.includes(code))) throw new FiscalCustomerError('response');
      return value;
    },
    async capabilities() {
      const value = await request('capabilities');
      if (!['transmission_available','nfse_available','sped_generation_available','production_cancellation_available'].every(key => typeof value[key] === 'boolean')) throw new FiscalCustomerError('response');
      return value;
    },
    async requirements(uf) {
      if (!states.includes(uf)) throw new FiscalCustomerError('invalid');
      const value = await request('requirements/' + uf);
      if (value.uf !== uf || !['reference_date','issuer_requirements','developer_requirements','limitations'].every(key => typeof value[key] === 'string') || !Array.isArray(value.sources) ||
          !value.sources.every(source => source && typeof source.title === 'string' && typeof source.url === 'string')) throw new FiscalCustomerError('response');
      return value;
    },
    async read(profileID, revision) {
      if (!id(profileID) || (revision !== undefined && (!Number.isSafeInteger(revision) || revision < 1))) throw new FiscalCustomerError('invalid');
      const value = validateProfile(await request('profiles/' + encodeURIComponent(profileID) + (revision === undefined ? '' : '?revision=' + revision)));
      if (value.input.profile_id !== profileID || (revision !== undefined && value.revision !== revision)) throw new FiscalCustomerError('response');
      return value;
    },
    async save(input) {
      if (Object.keys(input).sort().join() !== [...fields].sort().join() || !id(input.profile_id) || !id(input.operation_id) ||
          !Number.isSafeInteger(input.expected_revision) || input.expected_revision < 0) throw new FiscalCustomerError('invalid');
      const value = validateProfile(await request('profiles', input));
      if (!profileMatches(value, input)) throw new FiscalCustomerError('response');
      return value;
    },
  };
}
