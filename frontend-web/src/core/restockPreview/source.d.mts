import type {LocalSession,LocalCapabilities} from '../local/localClient.mjs';
import type {RestockHypothesis,RestockPreview,PreviewProduct} from './model.mjs';
export function createRestockPreview(options:{token:string;session:LocalSession;capabilities:LocalCapabilities;isCurrent:()=>boolean;fetcher?:typeof fetch}):{assertCurrent():void;preview(input:RestockHypothesis,product:PreviewProduct,signal?:AbortSignal):Promise<RestockPreview>};
