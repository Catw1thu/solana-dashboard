use super::model::{InstructionInput, InvocationSource, PumpfunInvocation};
use crate::{
    pumpfun::instruction::parse_instruction,
    transaction_view::{InnerInstructionView, OuterInstructionView, TransactionView},
};

fn build_input_from_outer(ix: &OuterInstructionView) -> InstructionInput {
    InstructionInput {
        program_id: ix.program_id.clone(),
        account_pubkeys: ix.account_pubkeys.clone(),
        data_base64: ix.data_base64.clone(),
    }
}

fn build_input_from_inner(ix: &InnerInstructionView) -> InstructionInput {
    InstructionInput {
        program_id: ix.program_id.clone(),
        account_pubkeys: ix.account_pubkeys.clone(),
        data_base64: ix.data_base64.clone(),
    }
}

pub fn extract_outer_invocations(view: &TransactionView) -> Vec<PumpfunInvocation> {
    let mut invocations = Vec::new();

    for (outer_index, ix) in view.outer_instructions.iter().enumerate() {
        let input = build_input_from_outer(ix);

        let Some(instruction) = parse_instruction(&input) else {
            continue;
        };

        invocations.push(PumpfunInvocation {
            source: InvocationSource::Outer { outer_index },
            instruction,
        });
    }
    invocations
}

pub fn extract_inner_invocations(view: &TransactionView) -> Vec<PumpfunInvocation> {
    let mut invocations = Vec::new();

    for group in &view.inner_instruction_groups {
        let outer_index = group.outer_instruction_index as usize;
        for (inner_index, ix) in group.instructions.iter().enumerate() {
            let input = build_input_from_inner(ix);

            let Some(instruction) = parse_instruction(&input) else {
                continue;
            };

            invocations.push(PumpfunInvocation {
                source: InvocationSource::Inner {
                    outer_index,
                    inner_index,
                },
                instruction,
            });
        }
    }
    invocations
}

pub fn extract_invocations(view: &TransactionView) -> Vec<PumpfunInvocation> {
    let mut invocations = extract_outer_invocations(view);
    invocations.extend(extract_inner_invocations(view));
    invocations
}
