// UI navigation mirrors server capabilities; it never grants API permissions.
export function allowedAreas(capabilities) {
 const p=new Set(capabilities?.permissions || []), m=new Set(capabilities?.license?.modules || []);
 const signed=['active','expired','not_yet_valid'].includes(capabilities?.license?.state);
 const has=(module,permission)=>signed && m.has(module) && p.has(permission);
 const areas=['home'];
 if(has('inventory','view_catalog')) areas.push('catalog','prices');
 if(has('inventory','manage_stock')) areas.push('stock');
 if(has('pos','sell')) areas.push('pos','cash','sos','payments');
 if(has('production','manage_production')) areas.push('production');
 if(has('orders','view_orders')) areas.push('orders');
 if(has('logistics','view_orders')) areas.push('fleet');
 if(has('staff','manage_staff')) areas.push('staff');
 if(signed && m.has('staff')) areas.push('point');
 if(has('accounting','view_accounting')) areas.push('accounting');
 if(capabilities?.role==='owner') areas.push('plans');
 return areas;
}
export function areaForRoute(pathname) {
 if(pathname==='/local/prices')return 'prices';
 if(pathname==="/local/orders")return "orders";
 if(pathname==='/local/pos')return 'pos';
 if(pathname==='/local/catalog')return 'catalog';
 if(pathname==='/local/stock')return 'stock';
 if(pathname==='/local/subscription')return 'plans';
 if(pathname==='/local/staff')return 'staff';
 return 'home';
}
