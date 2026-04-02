use super::model::{InstructionInput, InvocationSource, PumpAmmInvocation};
use crate::{
    pumpamm::instruction::parse_instruction,
    transaction_view::{
        InnerInstructionGroup, InnerInstructionView, OuterInstructionView, TransactionView,
    },
};

#[allow(dead_code)]
fn build_input_from_outer(ix: &OuterInstructionView) -> InstructionInput {
    InstructionInput {
        program_id: ix.program_id.clone(),
        account_pubkeys: ix.account_pubkeys.clone(),
        data_base64: ix.data_base64.clone(),
    }
}

#[allow(dead_code)]
fn build_input_from_inner(ix: &InnerInstructionView) -> InstructionInput {
    InstructionInput {
        program_id: ix.program_id.clone(),
        account_pubkeys: ix.account_pubkeys.clone(),
        data_base64: ix.data_base64.clone(),
    }
}

#[allow(dead_code)]
pub fn extract_invocations(view: &TransactionView) -> Vec<PumpAmmInvocation> {
    let mut pending_inner_groups = view
        .inner_instruction_groups
        .iter()
        .map(|group| (group.outer_instruction_index as usize, group))
        .collect::<std::collections::BTreeMap<_, _>>();

    let mut invocations = Vec::new();

    for (outer_index, outer_ix) in view.outer_instructions.iter().enumerate() {
        if let Some(instruction) = parse_instruction(&build_input_from_outer(outer_ix)) {
            invocations.push(PumpAmmInvocation {
                source: InvocationSource::Outer { outer_index },
                instruction,
            });
        }

        if let Some(group) = pending_inner_groups.remove(&outer_index) {
            collect_inner_group_invocations(group, outer_index, &mut invocations);
        }
    }

    for (outer_index, group) in pending_inner_groups {
        collect_inner_group_invocations(group, outer_index, &mut invocations);
    }

    invocations
}

#[allow(dead_code)]
fn collect_inner_group_invocations(
    group: &InnerInstructionGroup,
    outer_index: usize,
    invocations: &mut Vec<PumpAmmInvocation>,
) {
    for (inner_index, ix) in group.instructions.iter().enumerate() {
        let input = build_input_from_inner(ix);

        let Some(instruction) = parse_instruction(&input) else {
            continue;
        };

        invocations.push(PumpAmmInvocation {
            source: InvocationSource::Inner {
                outer_index,
                inner_index,
            },
            instruction,
        });
    }
}
