// Generated types from the exact read contracts. Runtime validation remains mandatory.
export type Appointment = {appointment_id:string;arrived_at:string | null;completed_at:string | null;created_at:string;created_by:string;dock_id:string;ends_at:string;order_id:string;revision:number;starts_at:string;status:string;stock_received:boolean;supplier_confirmed:boolean;supplier_id:string;unloading_at:string | null;updated_at:string;};
export type Dock = {created_at:string;dock_id:string;enabled:boolean;name:string;revision:number;updated_at:string;};
export type ExactSeconds = {denominator:string;numerator:string;};
export type FreeWindows = {active_unloading:Appointment | null;basis:string;confirmed_available:false;dock:Dock;duration_minutes:number;from:string;measured_at:string;occupied_windows:number;to:string;waiting_count:number;windows:(Gap)[];};
export type Gap = {ends_at:string;first_fit_ends_at:string;starts_at:string;};
export type PendingItem = {appointment:Appointment;order_cancelled:boolean;priority:number;reason:string;};
export type PendingPage = {items:(PendingItem)[];limit:number;measured_at:string;offset:number;total:number;};
export type Performance = {all_suppliers:SupplierMetrics;basis:string;from:string;items:(SupplierMetrics)[];limit:number;measured_at:string;offset:number;to:string;total_suppliers:number;};
export type SupplierMetrics = {appointments:number;arrival_samples:number;arrived:number;average_unloading_seconds:ExactSeconds | null;average_waiting_seconds:ExactSeconds | null;cancelled:number;completed:number;late_arrivals:number;no_show:number;reserved:number;supplier_id:string;unloading:number;unloading_samples:number;waiting_samples:number;};
export type OccupiedSegment = {appointment_id:string;ends_at:string;order_id:string;seconds:ExactSeconds;starts_at:string;status:string;still_unloading:boolean;supplier_id:string;};
export type UtilizationDay = {day_utc:string;from:string;period_seconds:ExactSeconds;planned_seconds:ExactSeconds;to:string;unloading_fraction:ExactSeconds;unloading_seconds:ExactSeconds;};
export type UtilizationReport = {basis:string;days:(UtilizationDay)[];dock:Dock;from:string;items:(OccupiedSegment)[];limit:number;measured_at:string;offset:number;period_seconds:ExactSeconds;planned_appointments:number;planned_fraction:ExactSeconds;planned_seconds:ExactSeconds;source_appointments:number;to:string;total_segments:number;unloading_fraction:ExactSeconds;unloading_seconds:ExactSeconds;without_registered_unloading_seconds:ExactSeconds;};
