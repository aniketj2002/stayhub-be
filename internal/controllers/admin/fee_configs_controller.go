package admin

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/stayhub/backend/internal/domain/apperrors"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/http/middleware"
	"github.com/stayhub/backend/internal/http/response"
	"github.com/stayhub/backend/internal/services"
)

type FeeConfigsController struct {
	feeService interfaces.FeeConfigService
}

func NewFeeConfigsController(feeService interfaces.FeeConfigService) *FeeConfigsController {
	return &FeeConfigsController{
		feeService: feeService,
	}
}

func (ctrl *FeeConfigsController) ListFeeConfigs(c *gin.Context) {
	configs, err := ctrl.feeService.List(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Failed to list fee configurations")
		return
	}

	response.OK(c, gin.H{
		"fee_configs": configs,
		"count":       len(configs),
	})
}

func (ctrl *FeeConfigsController) ProposeFeeConfig(c *gin.Context) {
	input, valErr := ValidateProposeFeeConfig(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	actor, ok := middleware.ActorFromGin(c)
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}

	cfg, err := ctrl.feeService.Propose(c.Request.Context(), actor.SubjectID, input.GuestFeeBPS, input.HostFeeBPS, input.Reason)
	if err != nil {
		if errors.Is(err, apperrors.ErrValidation) {
			response.Fail(c, http.StatusBadRequest, "PROPOSE_FAILED", err.Error(), nil)
			return
		}
		response.InternalError(c, "Failed to propose fee configuration")
		return
	}

	response.Created(c, cfg)
}

func (ctrl *FeeConfigsController) ApproveFeeConfig(c *gin.Context) {
	id, valErr := ParseUUIDParam(c, "id")
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	actor, ok := middleware.ActorFromGin(c)
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}

	cfg, err := ctrl.feeService.Approve(c.Request.Context(), actor.SubjectID, id)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Fee configuration not found")
			return
		}
		if errors.Is(err, services.ErrProposerCannotApprove) {
			response.Fail(c, http.StatusBadRequest, "APPROVER_CANNOT_BE_PROPOSER", err.Error(), nil)
			return
		}
		if errors.Is(err, services.ErrFeeConfigNotPending) {
			response.Fail(c, http.StatusBadRequest, "INVALID_STATUS", err.Error(), nil)
			return
		}
		response.InternalError(c, "Failed to approve fee configuration")
		return
	}

	response.OK(c, cfg)
}

func (ctrl *FeeConfigsController) RejectFeeConfig(c *gin.Context) {
	id, valErr := ParseUUIDParam(c, "id")
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	input, valErr := ValidateRejectFeeConfig(c)
	if valErr != nil {
		response.BadRequest(c, valErr.Code, valErr.Message, valErr.Fields)
		return
	}

	actor, ok := middleware.ActorFromGin(c)
	if !ok {
		response.Unauthorized(c, "Authentication required")
		return
	}

	cfg, err := ctrl.feeService.Reject(c.Request.Context(), actor.SubjectID, id, input.Reason)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			response.NotFound(c, "Fee configuration not found")
			return
		}
		if errors.Is(err, services.ErrFeeConfigNotPending) || errors.Is(err, apperrors.ErrValidation) {
			response.Fail(c, http.StatusBadRequest, "REJECT_FAILED", err.Error(), nil)
			return
		}
		response.InternalError(c, "Failed to reject fee configuration")
		return
	}

	response.OK(c, cfg)
}
