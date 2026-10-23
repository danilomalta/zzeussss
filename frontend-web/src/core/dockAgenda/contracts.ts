// Typed projection of the available local dock-agenda contract.
export interface ActionInput {
    action: "reschedule" | "arrive" | "start" | "complete" | "cancel" | "no_show";
    dock_id?: string;
    ends_at?: string;
    expected_revision: number;
    operation_id: string;
    reason: string;
    starts_at?: string;
}
export interface Appointment {
    appointment_id: string;
    arrived_at: string | null;
    completed_at: string | null;
    created_at: string;
    created_by: string;
    dock_id: string;
    ends_at: string;
    order_id: string;
    revision: number;
    starts_at: string;
    status: string;
    stock_received: false;
    supplier_confirmed: false;
    supplier_id: string;
    unloading_at: string | null;
    updated_at: string;
}
export interface BookInput {
    appointment_id: string;
    dock_id: string;
    ends_at: string;
    operation_id: string;
    order_id: string;
    reason: string;
    starts_at: string;
}
export interface Detail {
    appointment: Appointment;
    arrival_delay_seconds: number | null;
    measured_at: string;
    order_cancelled: boolean;
    unloading_seconds: number | null;
    waiting_seconds: number | null;
}
export interface Dock {
    created_at: string;
    dock_id: string;
    enabled: boolean;
    name: string;
    revision: number;
    updated_at: string;
}
export interface DockInput {
    dock_id: string;
    enabled: boolean;
    expected_revision: number;
    name: string;
    operation_id: string;
}
export interface Event {
    actor_id: string;
    device_id: string;
    request_json: string;
    result: Result;
}
export interface Occupancy {
    dock: Dock;
    measured_at: string;
    unloading: Appointment | null;
    waiting_count: number;
}
export interface Result {
    action: string;
    appointment?: Appointment | null;
    dock?: Dock | null;
    entity_id: string;
    entity_kind: string;
    occurred_at: string;
    operation_id: string;
    repeated: boolean;
    revision: number;
}
export interface DockPage {
    items: (Dock)[];
    total: number;
    limit: number;
    offset: number;
}
export interface AppointmentPage {
    items: (Appointment)[];
    total: number;
    limit: number;
    offset: number;
}
export interface EventPage {
    items: (Event)[];
    total: number;
    limit: number;
    offset: number;
}
export interface OccupancyPage {
    items: (Occupancy)[];
    total: number;
    limit: number;
    offset: number;
}
