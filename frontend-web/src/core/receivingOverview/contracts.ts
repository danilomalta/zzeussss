export interface AgendaCounts {
    arrived: number;
    cancelled: number;
    completed: number;
    no_show: number;
    reserved: number;
    unloading: number;
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
    stock_received: boolean;
    supplier_confirmed: boolean;
    supplier_id: string;
    unloading_at: string | null;
    updated_at: string;
}
export interface ReceivingAuthorization {
    actor_id: string;
    created_at: string;
    device_id: string;
    operation_id: string;
    reason: string;
    reference: string;
}
export interface ReceivingOverview {
    agenda_counts: AgendaCounts;
    appointments: (Appointment)[];
    authorization: ReceivingAuthorization | null;
    commercial_status: string;
    effective_accepted_milli: number;
    effective_delivered_milli: number;
    full_rejection_count: number;
    fully_rejected_milli_exact: string;
    limit: number;
    measured_at: string;
    offset: number;
    order_id: string;
    partial_rejected_milli: number;
    payment_evaluation: "not_evaluated";
    planned_milli: number;
    product_id: string;
    receipt_count: number;
    receiving_status: string;
    remaining_milli: number;
    review_flags: (string)[];
    source_records: number;
    supplier_confirmation: "not_proven_by_local_records";
    supplier_id: string;
    total_appointments: number;
    unit: string;
    voided_accepted_milli_exact: string;
    voided_receipt_count: number;
}
export interface BoardGroup {
    accepted_milli_exact: string;
    cancelled_orders: number;
    orders: number;
    pending_milli_exact: string;
    pending_orders: number;
    planned_milli_exact: string;
    product_id: string;
    received_orders: number;
    remaining_milli_exact: string;
    supplier_id: string;
    unit: string;
}
export interface BoardOrder {
    accepted_milli: number;
    commercial_status: string;
    created_at: string;
    order_id: string;
    planned_milli: number;
    product_id: string;
    receiving_status: string;
    remaining_milli: number;
    review_flags: (string)[];
    supplier_id: string;
    unit: string;
}
export interface ReceivingBoard {
    basis: string;
    cohort_orders: number;
    from: string;
    groups: (BoardGroup)[];
    items: (BoardOrder)[];
    limit: number;
    matched_orders: number;
    measured_at: string;
    offset: number;
    product_id: string;
    source_evidence: number;
    state: string;
    supplier_id: string;
    to: string;
}
